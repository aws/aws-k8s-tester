// Package nlbhelloworld implements NLB plugin
// with a simple hello world service.
package nlbhelloworld

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
	apierrs "k8s.io/apimachinery/pkg/api/errors"
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
	nlbHelloWorldDeploymentName = "hello-world-deployment"
	nlbHelloWorldAppName        = "hello-world"
	nlbHelloWorldAppImageName   = "dockercloud/hello-world"
	nlbHelloWorldServiceName    = "hello-world-service"
)

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNLBHelloWorld() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
	); err != nil {
		return err
	}
	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.waitDeployment(); err != nil {
		return err
	}
	if err := ts.createService(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNLBHelloWorld() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteService(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	/*
	   # NLB tags
	   kubernetes.io/service-name
	   leegyuho-test-prod-nlb-hello-world/hello-world-service

	   kubernetes.io/cluster/leegyuho-test-prod
	   owned
	*/
	if err := elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN,
		ts.cfg.EKSConfig.Parameters.VPCID,
		map[string]string{
			"kubernetes.io/cluster/" + ts.cfg.EKSConfig.Name: "owned",
			"kubernetes.io/service-name":                     ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace + "/" + nlbHelloWorldServiceName,
		},
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating NLB hello-world Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nlbHelloWorldDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": nlbHelloWorldAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": nlbHelloWorldAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": nlbHelloWorldAppName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            nlbHelloWorldAppName,
									Image:           nlbHelloWorldAppImageName,
									ImagePullPolicy: v1.PullAlways,
									Ports: []v1.ContainerPort{
										{
											Protocol:      v1.ProtocolTCP,
											ContainerPort: 80,
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
		return fmt.Errorf("failed to create NLB hello-world Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB hello-world Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting NLB hello-world Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Delete(
			ctx,
			nlbHelloWorldDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete NLB hello-world Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted NLB hello-world Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() (err error) {
	timeout := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = k8s_client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		20*time.Second,
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
		nlbHelloWorldDeploymentName,
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas,
		k8s_client.WithQueryFunc(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(
				ctx,
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace="+ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
				"describe",
				"deployment",
				nlbHelloWorldDeploymentName,
			).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating NLB hello-world Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Create(
			ctx,
			&v1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      nlbHelloWorldServiceName,
					Namespace: ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
					},
				},
				Spec: v1.ServiceSpec{
					Selector: map[string]string{
						"app.kubernetes.io/name": nlbHelloWorldAppName,
					},
					Type: v1.ServiceTypeLoadBalancer,
					Ports: []v1.ServicePort{
						{
							Protocol:   v1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create NLB hello-world Service (%v)", err)
	}
	ts.cfg.Logger.Info("created NLB hello-world Service")

	waitDur := 3 * time.Minute
	ts.cfg.Logger.Info("waiting for NLB hello-world Service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("NLB hello-world Service creation aborted")
	case <-time.After(waitDur):
	}

	args := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
		"describe",
		"svc",
		nlbHelloWorldServiceName,
	}
	argsCmd := strings.Join(args, " ")
	hostName := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("NLB hello-world Service creation aborted")
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

		ts.cfg.Logger.Info("querying NLB hello-world Service for HTTP endpoint")
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
			Get(ctx, nlbHelloWorldServiceName, metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get NLB hello-world Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"NLB hello-world Service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"NLB hello-world Service has been linked to LoadBalancer.Ingress",
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
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL = "http://" + hostName
	ts.cfg.EKSConfig.Sync()

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB hello-world ARN: %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world Name: %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world URL: %s\n\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)

	ts.cfg.Logger.Info("waiting before testing hello-world Service")
	time.Sleep(20 * time.Second)

	htmlChecked := false
	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("hello-world Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := httputil.ReadInsecure(ts.cfg.Logger, ioutil.Discard, ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB hello-world Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Fprintf(ts.cfg.LogWriter, "\nNLB hello-world Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<h1>Hello world!</h1>`) {
			ts.cfg.Logger.Info("read hello-world Service; exiting", zap.String("host-name", hostName))
			htmlChecked = true
			break
		}

		ts.cfg.Logger.Warn("unexpected hello-world Service output; retrying")
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB hello-world ARN: %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world Name: %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world URL: %s\n\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)

	if !htmlChecked {
		return fmt.Errorf("NLB hello-world %q did not return expected HTML output", ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteService() error {
	ts.cfg.Logger.Info("deleting NLB hello-world Service")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Delete(
			ctx,
			nlbHelloWorldServiceName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete NLB hello-world Service (%v)", err)
	}

	ts.cfg.Logger.Info("deleted NLB hello-world Service", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}
