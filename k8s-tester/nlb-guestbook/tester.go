// Package nlb_guestbook installs a simple guestbook application with NLB.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/nlb-guestbook.
package nlb_guestbook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	aws_v1 "github.com/aws/aws-k8s-tester/utils/aws/v1"
	aws_v1_elb "github.com/aws/aws-k8s-tester/utils/aws/v1/elb"
	"github.com/aws/aws-k8s-tester/utils/http"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/exec"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	ELB2API elbv2iface.ELBV2API `json:"-"`

	AccountID string `json:"account_id" read-only:"true"`
	Partition string `json:"partition"`
	Region    string `json:"region"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	// DeploymentNodeSelector is configured to overwrite existing node selector
	// for hello world deployment. If left empty, tester sets default selector.
	DeploymentNodeSelector map[string]string `json:"deployment_node_selector"`
	// DeploymentReplicas is the number of replicas to deploy using "Deployment" object.
	DeploymentReplicas int32 `json:"deployment_replicas"`

	// ELBARN is the ARN of the ELB created from the service.
	ELBARN string `json:"elb_arn" read-only:"true"`
	// ELBName is the name of the ELB created from the service.
	ELBName string `json:"elb_name" read-only:"true"`
	// ELBURL is the host name for guestbook service.
	ELBURL string `json:"elb_url" read-only:"true"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}

	return nil
}

const (
	DefaultMinimumNodes       int   = 1
	DefaultDeploymentReplicas int32 = 2
)

func NewDefault() *Config {
	return &Config{
		Enable:             false,
		Prompt:             false,
		MinimumNodes:       DefaultMinimumNodes,
		Namespace:          pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		DeploymentReplicas: DefaultDeploymentReplicas,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	awsCfg := aws_v1.Config{
		Logger:        cfg.Logger,
		DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
		Partition:     cfg.Partition,
		Region:        cfg.Region,
	}
	awsSession, stsOutput, _, err := aws_v1.New(&awsCfg)
	if err != nil {
		panic(err)
	}
	cfg.ELB2API = elbv2.New(awsSession)
	if cfg.AccountID == "" && stsOutput.Account != nil {
		cfg.AccountID = *stsOutput.Account
	}

	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

const (
	redisLabelName = "redis"

	redisLeaderDeploymentName = "redis-leader"
	redisLeaderAppName        = "redis-master"
	// ref. https://hub.docker.com/_/redis/?tab=tags
	// ref. https://gallery.ecr.aws/bitnami/redis
	redisLeaderAppImageName = "public.ecr.aws/bitnami/redis:latest"
	redisLeaderServiceName  = "redis-master" // e..g "Connecting to MASTER redis-master:6379"
	redisLeaderRoleName     = "master"       // TODO: change this to "leader"

	redisFollowerDeploymentName = "redis-follower"
	redisFollowerAppName        = "redis-slave"
	// ref. https://hub.docker.com/_/redis/?tab=tags
	redisFollowerAppImageName = "k8s.gcr.io/redis-slave:v2"
	redisFollowerServiceName  = "redis-slave"
	redisFollowerRoleName     = "slave" // TODO: change this to "follower"

	deploymentName = "guestbook"
	appName        = "guestbook"
	appImageName   = "k8s.gcr.io/guestbook:v3"
	serviceName    = "guestbook"
)

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if ts.cfg.MinimumNodes > 0 {
		if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
			return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
		}
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createDeploymentRedisLeader(); err != nil {
		return err
	}
	if err := ts.checkDeploymentRedisLeader(); err != nil {
		return err
	}
	if err := ts.createServiceRedisLeader(); err != nil {
		return err
	}
	if err := ts.checkServiceRedis(redisLeaderServiceName); err != nil {
		return err
	}

	if err := ts.createDeploymentRedisFollower(); err != nil {
		return err
	}
	if err := ts.checkDeploymentRedisFollower(); err != nil {
		return err
	}
	if err := ts.createServiceRedisFollower(); err != nil {
		return err
	}
	if err := ts.checkServiceRedis(redisFollowerServiceName); err != nil {
		return err
	}

	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.checkDeployment(); err != nil {
		return err
	}
	if err := ts.createService(); err != nil {
		return err
	}

	waitDur := 3 * time.Minute
	ts.cfg.Logger.Info("waiting for NLB guestbook Service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("NLB guestbook Service apply aborted")
	case <-time.After(waitDur):
	}

	if err := ts.checkService(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	// get ELB ARN before deleting the service
	if ts.cfg.ELBARN == "" {
		_, elbARN, elbName, exists, err := client.FindServiceIngressHostname(
			ts.cfg.Logger,
			ts.cfg.Client.KubernetesClient(),
			ts.cfg.Namespace,
			serviceName,
			ts.cfg.Stopc,
			3*time.Minute,
			ts.cfg.AccountID,
			ts.cfg.Region,
		)
		if err != nil {
			if exists { // maybe already deleted from previous run
				errs = append(errs, fmt.Sprintf("ELB exists but failed to find ingress ELB ARN (%v)", err))
			}
		}
		ts.cfg.ELBARN = elbARN
		ts.cfg.ELBName = elbName
	}

	ts.cfg.Logger.Info("deleting service", zap.String("service-name", serviceName))
	if err := client.DeleteService(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		serviceName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)

	ts.cfg.Logger.Info("deleting deployment", zap.String("deployment-name", deploymentName))
	if err := client.DeleteDeployment(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		deploymentName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	/*
	  proactively delete ELB resource
	  ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/nlb-guestbook/nlb-guestbook.go#L135-L154

	  # NLB tags
	  kubernetes.io/service-name
	  leegyuho-test-prod-nlb-guestbook/guestbook-service
	  kubernetes.io/cluster/leegyuho-test-prod
	  owned
	*/
	if ts.cfg.ELBARN != "" {
		if err := aws_v1_elb.DeleteELBv2(
			ts.cfg.Logger,
			ts.cfg.ELB2API,
			ts.cfg.ELBARN,
		); err != nil {
			errs = append(errs, fmt.Sprintf("failed to delete ELB (%v)", err))
		}
	}

	ts.cfg.Logger.Info("deleting service redis follower", zap.String("service-name", redisFollowerServiceName))
	if err := client.DeleteService(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		redisFollowerServiceName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete service redis follower (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting service redis follower")
	time.Sleep(time.Minute)

	ts.cfg.Logger.Info("deleting deployment redis follower", zap.String("deployment-name", redisFollowerDeploymentName))
	if err := client.DeleteDeployment(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		redisFollowerDeploymentName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete deployment redis follower (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting deployment redis follower")
	time.Sleep(time.Minute)

	ts.cfg.Logger.Info("deleting service redis leader", zap.String("service-name", redisLeaderServiceName))
	if err := client.DeleteService(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		redisLeaderServiceName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete service redis leader (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting service redis leader")
	time.Sleep(time.Minute)

	ts.cfg.Logger.Info("deleting deployment redis leader", zap.String("deployment-name", redisLeaderDeploymentName))
	if err := client.DeleteDeployment(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		redisLeaderDeploymentName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete deployment redis leader (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting deployment redis leader")
	time.Sleep(time.Minute)

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

func (ts *tester) createDeploymentRedisLeader() error {
	var nodeSelector map[string]string
	if len(ts.cfg.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating redis leader Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		AppsV1().
		Deployments(ts.cfg.Namespace).
		Create(
			ctx,
			&apps_v1.Deployment{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      redisLeaderDeploymentName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
				},
				Spec: apps_v1.DeploymentSpec{
					Replicas: int32Ref(1),
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": redisLabelName,
							"role":                   redisLeaderRoleName,
						},
					},
					Template: core_v1.PodTemplateSpec{
						ObjectMeta: meta_v1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": redisLabelName,
								"role":                   redisLeaderRoleName,
							},
						},
						Spec: core_v1.PodSpec{
							RestartPolicy: core_v1.RestartPolicyAlways,
							Containers: []core_v1.Container{
								{
									Name:            redisLeaderAppName,
									Image:           redisLeaderAppImageName,
									ImagePullPolicy: core_v1.PullAlways,
									Ports: []core_v1.ContainerPort{
										{
											Name:          "redis-server",
											Protocol:      core_v1.ProtocolTCP,
											ContainerPort: 6379,
										},
									},
									Env: []core_v1.EnvVar{
										{
											Name:  "ALLOW_EMPTY_PASSWORD",
											Value: "yes",
										},
									},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("redis leader Deployment already exists")
			return nil
		}
		return fmt.Errorf("failed to create redis leader Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created redis leader Deployment")
	return nil
}

func (ts *tester) checkDeploymentRedisLeader() error {
	timeout := 7*time.Minute + time.Duration(ts.cfg.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDeploymentAvailables(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.Client.KubernetesClient(),
		time.Minute,
		20*time.Second,
		ts.cfg.Namespace,
		redisLeaderDeploymentName,
		ts.cfg.DeploymentReplicas,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"deployment",
				redisLeaderDeploymentName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)

			logsArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"logs",
				"--selector=app.kubernetes.io/name=" + redisLabelName + ",role=" + redisLeaderRoleName,
				"--timestamps",
			}
			logsCmd := strings.Join(logsArgs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			}
			out = string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", logsCmd, out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) createServiceRedisLeader() error {
	ts.cfg.Logger.Info("creating redis leader Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		CoreV1().
		Services(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.Service{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      redisLeaderServiceName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
				},
				Spec: core_v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisLeaderRoleName,
					},
					Type: core_v1.ServiceTypeClusterIP,
					Ports: []core_v1.ServicePort{
						{
							Protocol:   core_v1.ProtocolTCP,
							Port:       6379,
							TargetPort: intstr.FromString("redis-server"),
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("redis leader Service already exists")
			return nil
		}
		return fmt.Errorf("failed to create redis leader Service (%v)", err)
	}

	ts.cfg.Logger.Info("created redis leader Service")
	return nil
}

func (ts *tester) createDeploymentRedisFollower() error {
	var nodeSelector map[string]string
	if len(ts.cfg.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating redis follower Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		AppsV1().
		Deployments(ts.cfg.Namespace).
		Create(
			ctx,
			&apps_v1.Deployment{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      redisFollowerDeploymentName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
				},
				Spec: apps_v1.DeploymentSpec{
					Replicas: int32Ref(2),
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": redisLabelName,
							"role":                   redisFollowerRoleName,
						},
					},
					Template: core_v1.PodTemplateSpec{
						ObjectMeta: meta_v1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": redisLabelName,
								"role":                   redisFollowerRoleName,
							},
						},
						Spec: core_v1.PodSpec{
							RestartPolicy: core_v1.RestartPolicyAlways,
							Containers: []core_v1.Container{
								{
									Name:            redisFollowerAppName,
									Image:           redisFollowerAppImageName,
									ImagePullPolicy: core_v1.PullAlways,
									Ports: []core_v1.ContainerPort{
										{
											Name:          "redis-server",
											Protocol:      core_v1.ProtocolTCP,
											ContainerPort: 6379,
										},
									},
									Env: []core_v1.EnvVar{
										{
											Name:  "ALLOW_EMPTY_PASSWORD",
											Value: "yes",
										},
									},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("redis follower Deployment already exists")
			return nil
		}
		return fmt.Errorf("failed to create redis follower Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created redis follower Deployment")
	return nil
}

func (ts *tester) checkDeploymentRedisFollower() error {
	timeout := 7*time.Minute + time.Duration(ts.cfg.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDeploymentAvailables(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.Client.KubernetesClient(),
		time.Minute,
		20*time.Second,
		ts.cfg.Namespace,
		redisFollowerDeploymentName,
		ts.cfg.DeploymentReplicas,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"deployment",
				redisFollowerDeploymentName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)

			logsArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"logs",
				"--selector=app.kubernetes.io/name=" + redisLabelName + ",role=" + redisFollowerRoleName,
				"--timestamps",
			}
			logsCmd := strings.Join(logsArgs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			}
			out = string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", logsCmd, out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) createServiceRedisFollower() error {
	ts.cfg.Logger.Info("creating redis follower Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		CoreV1().
		Services(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.Service{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      redisFollowerServiceName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
				},
				Spec: core_v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": redisLabelName,
						"role":                   redisFollowerRoleName,
					},
					Type: core_v1.ServiceTypeClusterIP,
					Ports: []core_v1.ServicePort{
						{
							Protocol:   core_v1.ProtocolTCP,
							Port:       6379,
							TargetPort: intstr.FromString("redis-server"),
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("redis follower Service already exists")
			return nil
		}
		return fmt.Errorf("failed to create redis follower Service (%v)", err)
	}

	ts.cfg.Logger.Info("created redis follower Service")
	return nil
}

func (ts *tester) checkServiceRedis(svcName string) (err error) {
	args := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"describe",
		"svc",
		svcName,
	}
	argsCmd := strings.Join(args, " ")

	waitDur := 3 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("redis service check aborted")
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

		ts.cfg.Logger.Info("querying redis service", zap.String("service-name", svcName))
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		_, err = ts.cfg.Client.KubernetesClient().
			CoreV1().
			Services(ts.cfg.Namespace).
			Get(ctx, svcName, meta_v1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get redis service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info("redis service is ready", zap.String("service-name", svcName))
		break
	}

	return nil
}

func (ts *tester) createDeployment() error {
	var nodeSelector map[string]string
	if len(ts.cfg.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating NLB guestbook Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		AppsV1().
		Deployments(ts.cfg.Namespace).
		Create(
			ctx,
			&apps_v1.Deployment{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      deploymentName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				Spec: apps_v1.DeploymentSpec{
					Replicas: &ts.cfg.DeploymentReplicas,
					Selector: &meta_v1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": appName,
						},
					},
					Template: core_v1.PodTemplateSpec{
						ObjectMeta: meta_v1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": appName,
							},
						},
						Spec: core_v1.PodSpec{
							RestartPolicy: core_v1.RestartPolicyAlways,
							Containers: []core_v1.Container{
								{
									Name:            appName,
									Image:           appImageName,
									ImagePullPolicy: core_v1.PullAlways,
									Ports: []core_v1.ContainerPort{
										{
											Protocol:      core_v1.ProtocolTCP,
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
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("NLB guestbook Deployment already exists")
			return nil
		}
		return fmt.Errorf("failed to create NLB guestbook Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB guestbook Deployment")
	return nil
}

func (ts *tester) checkDeployment() error {
	timeout := 7*time.Minute + time.Duration(ts.cfg.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDeploymentAvailables(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.Client.KubernetesClient(),
		time.Minute,
		20*time.Second,
		ts.cfg.Namespace,
		deploymentName,
		ts.cfg.DeploymentReplicas,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"describe",
				"deployment",
				deploymentName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)

			logsArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"logs",
				"--selector=app.kubernetes.io/name=" + appName,
				"--timestamps",
			}
			logsCmd := strings.Join(logsArgs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			}
			out = string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", logsCmd, out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating NLB guestbook Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		CoreV1().
		Services(ts.cfg.Namespace).
		Create(
			ctx,
			&core_v1.Service{
				TypeMeta: meta_v1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      serviceName,
					Namespace: ts.cfg.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": appName,
					},
				},
				Spec: core_v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": appName,
					},
					Type: core_v1.ServiceTypeLoadBalancer,
					Ports: []core_v1.ServicePort{
						{
							Protocol:   core_v1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromString("http-server"),
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("NLB guestbook Service already exists")
			return nil
		}
		return fmt.Errorf("failed to create NLB guestbook Service (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB guestbook Service")
	return nil
}

func (ts *tester) checkService() (err error) {
	queryFunc := func() {
		args := []string{
			ts.cfg.Client.Config().KubectlPath,
			"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
			"--namespace=" + ts.cfg.Namespace,
			"describe",
			"svc",
			serviceName,
		}
		argsCmd := strings.Join(args, " ")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmdOut, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.String("command", argsCmd), zap.Error(err))
		} else {
			out := string(cmdOut)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", argsCmd, out)
		}
	}

	hostName, elbARN, elbName, err := client.WaitForServiceIngressHostname(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		serviceName,
		ts.cfg.Stopc,
		3*time.Minute,
		ts.cfg.AccountID,
		ts.cfg.Region,
		client.WithQueryFunc(queryFunc),
	)
	if err != nil {
		return err
	}
	elbURL := "http://" + hostName

	ts.cfg.ELBARN = elbARN
	ts.cfg.ELBName = elbName
	ts.cfg.ELBURL = elbURL

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB guestbook ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook URL: %s\n\n", elbURL)

	ts.cfg.Logger.Info("waiting before testing guestbook Service")
	time.Sleep(20 * time.Second)

	htmlChecked := false
	retryStart := time.Now()
	for time.Since(retryStart) < 3*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("guestbook Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := http.ReadInsecure(ts.cfg.Logger, ioutil.Discard, elbURL)
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

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB guestbook ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB guestbook URL: %s\n\n", elbURL)

	if !htmlChecked {
		return fmt.Errorf("NLB guestbook %q did not return expected HTML output", elbURL)
	}

	return nil
}

func int32Ref(v int32) *int32 {
	return &v
}
