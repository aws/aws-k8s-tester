// Package nlb_hello_world installs a simple "Hello World" application with NLB.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/nlb-hello-world.
package nlb_hello_world

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

	DeploymentNodeSelector map[string]string `json:"deployment_node_selector"`
	DeploymentReplicas     int32             `json:"deployment_replicas"`

	// ELBName is the name of the ELB created from the service.
	ELBName string `json:"elb_name" read-only:"true"`
	// ELBARN is the ARN of the ELB created from the service.
	ELBARN string `json:"elb_arn" read-only:"true"`
	// ELBURL is the host name for hello-world service.
	ELBURL string `json:"elb_url" read-only:"true"`
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
	deploymentName = "hello-world-deployment"
	appName        = "hello-world"
	appImageName   = "dockercloud/hello-world"
	serviceName    = "hello-world-service"
)

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cfg.Client); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client, ts.cfg.Namespace); err != nil {
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
	ts.cfg.Logger.Info("waiting for NLB hello-world Service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("NLB hello-world Service apply aborted")
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
			ts.cfg.Client,
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

	if err := client.DeleteService(
		ts.cfg.Logger,
		ts.cfg.Client,
		ts.cfg.Namespace,
		serviceName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)

	if err := client.DeleteDeployment(
		ts.cfg.Logger,
		ts.cfg.Client,
		ts.cfg.Namespace,
		deploymentName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	/*
	  proactively delete ELB resource
	  ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/nlb-hello-world/nlb-hello-world.go#L135-L154

	  # NLB tags
	  kubernetes.io/service-name
	  leegyuho-test-prod-nlb-hello-world/hello-world-service
	  kubernetes.io/cluster/leegyuho-test-prod
	  owned
	*/
	if err := aws_v1_elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.ELBARN,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ELB (%v)", err))
	}

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client,
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

func (ts *tester) createDeployment() error {
	var nodeSelector map[string]string
	if len(ts.cfg.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating NLB hello-world Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.
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
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("NLB hello-world Deployment already exists")
			return nil
		}
		return fmt.Errorf("failed to create NLB hello-world Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB hello-world Deployment")
	return nil
}

func (ts *tester) checkDeployment() error {
	timeout := 7*time.Minute + time.Duration(ts.cfg.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.Client,
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
		}),
	)
	cancel()
	return err
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating NLB hello-world Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.
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
					Annotations: map[string]string{
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
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
							TargetPort: intstr.FromInt(80),
						},
					},
				},
			},
			meta_v1.CreateOptions{},
		)
	cancel()
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("NLB hello-world Service already exists")
			return nil
		}
		return fmt.Errorf("failed to create NLB hello-world Service (%v)", err)
	}

	ts.cfg.Logger.Info("created NLB hello-world Service")
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
		ts.cfg.Client,
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

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB hello-world ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world URL: %s\n\n", elbURL)

	ts.cfg.Logger.Info("waiting before testing hello-world Service")
	time.Sleep(20 * time.Second)

	htmlChecked := false
	retryStart := time.Now()
	for time.Since(retryStart) < 3*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("hello-world Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := http.ReadInsecure(ts.cfg.Logger, ioutil.Discard, elbURL)
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

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB hello-world ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB hello-world URL: %s\n\n", elbURL)

	if !htmlChecked {
		return fmt.Errorf("NLB hello-world %q did not return expected HTML output", elbURL)
	}

	return nil
}
