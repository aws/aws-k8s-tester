// Package nlbguestbook implements NLB plugin
// with a simple guestbook service.
// ref. https://github.com/kubernetes/examples/tree/master/guestbook-go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/eks-guestbook.html
package nlbguestbook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/elb"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/exec"
)

// Config defines NLB configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	ELB2API   elbv2iface.ELBV2API
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

const (
	redisLabelName = "redis"

	redisLeaderDeploymentName = "redis-leader"
	redisLeaderAppName        = "redis-master"
	redisLeaderAppImageName   = "redis:2.8.23" // ref. https://hub.docker.com/_/redis/?tab=tags
	redisLeaderServiceName    = "redis-master" // e..g "Connecting to MASTER redis-master:6379"
	redisLeaderRoleName       = "master"       // TODO: change this to "leader"

	redisFollowerDeploymentName = "redis-follower"
	redisFollowerAppName        = "redis-slave"
	redisFollowerAppImageName   = "k8s.gcr.io/redis-slave:v2" // ref. https://hub.docker.com/_/redis/?tab=tags
	redisFollowerServiceName    = "redis-slave"
	redisFollowerRoleName       = "slave" // TODO: change this to "leader"

	nlbGuestbookDeploymentName = "guestbook"
	nlbGuestbookAppName        = "guestbook"
	nlbGuestbookAppImageName   = "k8s.gcr.io/guestbook:v3"
	nlbGuestbookServiceName    = "guestbook"
)

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNLBGuestbook() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnNLBGuestbook.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnNLBGuestbook.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNLBGuestbook.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
	); err != nil {
		return err
	}

	if err := ts.createDeploymentRedisLeader(); err != nil {
		return err
	}
	if err := ts.waitDeploymentRedisLeader(); err != nil {
		return err
	}
	if err := ts.createServiceRedisLeader(); err != nil {
		return err
	}

	if err := ts.createDeploymentRedisFollower(); err != nil {
		return err
	}
	if err := ts.waitDeploymentRedisFollower(); err != nil {
		return err
	}
	if err := ts.createServiceRedisFollower(); err != nil {
		return err
	}

	if err := ts.createDeploymentGuestbook(); err != nil {
		return err
	}
	if err := ts.waitDeploymentGuestbook(); err != nil {
		return err
	}
	if err := ts.createServiceGuestbook(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNLBGuestbook() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnNLBGuestbook.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNLBGuestbook.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteServiceGuestbook(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB guestbook Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)
	if err := ts.deleteDeploymentGuestbook(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB guestbook Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	if err := ts.deleteServiceRedisFollower(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete redis follower Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)
	if err := ts.deleteDeploymentRedisFollower(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete redis follower Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	if err := ts.deleteServiceRedisLeader(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete redis leader Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)
	if err := ts.deleteDeploymentRedisLeader(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete redis leader Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	/*
	   # NLB tags
	   kubernetes.io/service-name
	   leegyuho-test-prod-nlb-guestbook/guestbook-service

	   kubernetes.io/cluster/leegyuho-test-prod
	   owned
	*/
	if err := elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBARN,
		ts.cfg.EKSConfig.Parameters.VPCID,
		map[string]string{
			"kubernetes.io/cluster/" + ts.cfg.EKSConfig.Name: "owned",
			"kubernetes.io/service-name":                     ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace + "/" + nlbGuestbookServiceName,
		},
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB guestbook (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnNLBGuestbook.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeploymentRedisLeader() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating redis leader Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisLeaderDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": redisLabelName,
							"role":                   redisLeaderRoleName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": redisLabelName,
								"role":                   redisLeaderRoleName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            redisLeaderAppName,
									Image:           redisLeaderAppImageName,
									ImagePullPolicy: v1.PullAlways,
									Ports: []v1.ContainerPort{
										{
											Name:          "redis-server",
											Protocol:      v1.ProtocolTCP,
											ContainerPort: 6379,
										},
									},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create redis leader Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created redis leader Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeploymentRedisLeader() error {
	ts.cfg.Logger.Info("deleting redis leader Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			redisLeaderDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete redis leader Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted redis leader Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeploymentRedisLeader() error {
	ts.cfg.Logger.Info("waiting for redis leader Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"deployment",
		redisLeaderDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"logs",
		"--selector=app.kubernetes.io/name=" + redisLabelName + ",role=" + redisLeaderRoleName,
	}
	logsCmd := strings.Join(logsArgs, " ")

	ready := false
	waitDur := 7 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ts.cfg.Logger.Info("fetching pod logs", zap.String("logs-command", logsCmd))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		logsOutput, err := exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		out := string(logsOutput)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", logsCmd, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, redisLeaderDeploymentName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)

		// TODO: remove the pod with "Error: ImagePullBackOff"
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		// TODO: remove this hack and handle "Error: ImagePullBackOff"
		if available && dresp.Status.AvailableReplicas+1 >= 1 {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for redis leader Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createServiceRedisLeader() error {
	ts.cfg.Logger.Info("creating redis leader Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&v1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisLeaderServiceName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
					Type: v1.ServiceTypeClusterIP,
					Ports: []v1.ServicePort{
						{
							Protocol:   v1.ProtocolTCP,
							Port:       6379,
							TargetPort: intstr.FromString("redis-server"),
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create redis leader Service (%v)", err)
	}
	ts.cfg.Logger.Info("created redis leader Service")

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"svc",
		redisLeaderServiceName,
	}
	argsCmd := strings.Join(args, " ")

	waitDur := 3 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("redis leader Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}

		ts.cfg.Logger.Info("querying redis leader Service")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		_, err = ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, redisLeaderServiceName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get redis leader Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info("redis leader Service is ready")
		break
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceRedisLeader() error {
	ts.cfg.Logger.Info("deleting redis leader Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			redisLeaderServiceName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete redis leader Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted redis leader Service", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeploymentRedisFollower() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating redis follower Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisFollowerDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": redisLabelName,
							"role":                   redisFollowerRoleName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": redisLabelName,
								"role":                   redisFollowerRoleName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            redisFollowerAppName,
									Image:           redisFollowerAppImageName,
									ImagePullPolicy: v1.PullAlways,
									Ports: []v1.ContainerPort{
										{
											Name:          "redis-server",
											Protocol:      v1.ProtocolTCP,
											ContainerPort: 6379,
										},
									},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create redis follower Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created redis follower Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeploymentRedisFollower() error {
	ts.cfg.Logger.Info("deleting redis follower Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			redisFollowerDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete redis follower Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted redis follower Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeploymentRedisFollower() error {
	ts.cfg.Logger.Info("waiting for redis follower Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"deployment",
		redisFollowerDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"logs",
		"--selector=app.kubernetes.io/name=" + redisLabelName + ",role=" + redisLeaderRoleName,
	}
	logsCmd := strings.Join(logsArgs, " ")

	ready := false
	waitDur := 7 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ts.cfg.Logger.Info("fetching pod logs", zap.String("logs-command", logsCmd))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		logsOutput, err := exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		out := string(logsOutput)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", logsCmd, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, redisFollowerDeploymentName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)

		// TODO: remove the pod with "Error: ImagePullBackOff"
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		// TODO: remove this hack and handle "Error: ImagePullBackOff"
		if available && dresp.Status.AvailableReplicas >= 2 {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for redis follower Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createServiceRedisFollower() error {
	ts.cfg.Logger.Info("creating redis follower Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&v1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisFollowerServiceName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
					Type: v1.ServiceTypeClusterIP,
					Ports: []v1.ServicePort{
						{
							Protocol:   v1.ProtocolTCP,
							Port:       6379,
							TargetPort: intstr.FromString("redis-server"),
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create redis follower Service (%v)", err)
	}
	ts.cfg.Logger.Info("created redis follower Service")

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"svc",
		redisFollowerServiceName,
	}
	argsCmd := strings.Join(args, " ")

	waitDur := 3 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("redis follower Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}

		ts.cfg.Logger.Info("querying redis follower Service")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		_, err = ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, redisFollowerServiceName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get redis follower Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info("redis follower Service is ready")
		break
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceRedisFollower() error {
	ts.cfg.Logger.Info("deleting redis follower Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			redisFollowerServiceName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete redis follower Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted redis follower Service", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeploymentGuestbook() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating NLB guestbook Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nlbGuestbookDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": nlbGuestbookAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": nlbGuestbookAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": nlbGuestbookAppName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            nlbGuestbookAppName,
									Image:           nlbGuestbookAppImageName,
									ImagePullPolicy: v1.PullAlways,
									Ports: []v1.ContainerPort{
										{
											Protocol:      v1.ProtocolTCP,
											ContainerPort: 3000,
											Name:          "http-server",
										},
									},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create NLB guestbook Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB guestbook Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeploymentGuestbook() error {
	ts.cfg.Logger.Info("deleting NLB guestbook Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			nlbGuestbookDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete NLB guestbook Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted NLB guestbook Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeploymentGuestbook() error {
	ts.cfg.Logger.Info("waiting for NLB guestbook Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"deployment",
		nlbGuestbookDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"logs",
		"--selector=app.kubernetes.io/name=" + nlbGuestbookAppName,
	}
	logsCmd := strings.Join(logsArgs, " ")

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentReplicas)*time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ts.cfg.Logger.Info("fetching pod logs", zap.String("logs-command", logsCmd))
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		logsOutput, err := exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		out := string(logsOutput)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\n\"%s\" output:\n\n%s\n\n", logsCmd, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, nlbGuestbookDeploymentName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)

		// TODO: remove the pod with "Error: ImagePullBackOff"
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		// TODO: remove this hack and handle "Error: ImagePullBackOff"
		if available && dresp.Status.AvailableReplicas+1 >= ts.cfg.EKSConfig.AddOnNLBGuestbook.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for NLB guestbook Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createServiceGuestbook() error {
	ts.cfg.Logger.Info("creating NLB guestbook Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Create(
			ctx,
			&v1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nlbGuestbookServiceName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": nlbGuestbookAppName,
					},
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": nlbGuestbookAppName,
					},
					Type: v1.ServiceTypeLoadBalancer,
					Ports: []v1.ServicePort{
						{
							Protocol:   v1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromString("http-server"),
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create NLB guestbook Service (%v)", err)
	}
	ts.cfg.Logger.Info("created NLB guestbook Service")

	waitDur := 3 * time.Minute
	ts.cfg.Logger.Info("waiting for NLB guestbook Service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("NLB guestbook Service creation aborted")
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace,
		"describe",
		"svc",
		nlbGuestbookServiceName,
	}
	argsCmd := strings.Join(args, " ")

	hostName := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("NLB guestbook Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}

		ts.cfg.Logger.Info("querying NLB guestbook Service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
			Get(ctx, nlbGuestbookServiceName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get NLB guestbook Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"NLB guestbook Service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"NLB guestbook Service has been linked to LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}

		if hostName != "" {
			ts.cfg.Logger.Info("found NLB host name", zap.String("host-name", hostName))
			break
		}
	}

	if hostName == "" {
		return errors.New("failed to find NLB host name")
	}

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)
	ts.cfg.EKSConfig.AddOnNLBGuestbook.URL = "http://" + hostName
	ts.cfg.EKSConfig.Sync()

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB guestbook ARN: %s\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook Name: %s\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook URL: %s\n\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.URL)

	ts.cfg.Logger.Info("waiting before testing guestbook Service")
	time.Sleep(20 * time.Second)

	htmlChecked := false
	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("guestbook Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, ioutil.Discard, ts.cfg.EKSConfig.AddOnNLBGuestbook.URL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB guestbook Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Fprintf(ts.cfg.LogWriter, "\nNLB guestbook Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<h1>Guestbook</h1>`) {
			ts.cfg.Logger.Info("read guestbook Service; exiting", zap.String("host-name", hostName))
			htmlChecked = true
			break
		}

		ts.cfg.Logger.Warn("unexpected guestbook Service output; retrying")
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB guestbook ARN: %s\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook Name: %s\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.NLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook URL: %s\n\n", ts.cfg.EKSConfig.AddOnNLBGuestbook.URL)

	if !htmlChecked {
		return fmt.Errorf("NLB hello-world %q did not return expected HTML output", ts.cfg.EKSConfig.AddOnNLBGuestbook.URL)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceGuestbook() error {
	ts.cfg.Logger.Info("deleting NLB guestbook Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBGuestbook.Namespace).
		Delete(
			ctx,
			nlbGuestbookServiceName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete NLB guestbook Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted NLB guestbook Service", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}
