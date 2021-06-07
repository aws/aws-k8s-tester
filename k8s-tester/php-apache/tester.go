// Package php_apache installs a simple PHP Apache application.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/php-apache.
package php_apache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	aws_v1 "github.com/aws/aws-k8s-tester/utils/aws/v1"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`

	// Repository defines a custom ECR image repository.
	// For "php-apache".
	Repository *aws_v1_ecr.Repository `json:"repository,omitempty"`

	// DeploymentNodeSelector is configured to overwrite existing node selector
	// for PHP Apache deployment. If left empty, tester sets default selector.
	DeploymentNodeSelector map[string]string `json:"deployment_node_selector"`
	// DeploymentReplicas is the number of replicas to deploy using "Deployment" object.
	DeploymentReplicas int32 `json:"deployment_replicas"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}

	return nil
}

const (
	DefaultMinimumNodes       int   = 1
	DefaultDeploymentReplicas int32 = 3
)

func NewDefault() *Config {
	return &Config{
		Enable:             false,
		Prompt:             false,
		MinimumNodes:       DefaultMinimumNodes,
		Namespace:          pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		Repository:         &aws_v1_ecr.Repository{},
		DeploymentReplicas: DefaultDeploymentReplicas,
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ts := &tester{
		cfg: cfg,
	}
	if !cfg.Repository.IsEmpty() {
		awsCfg := aws_v1.Config{
			Logger:        cfg.Logger,
			DebugAPICalls: cfg.Logger.Core().Enabled(zapcore.DebugLevel),
			Partition:     cfg.Repository.Partition,
			Region:        cfg.Repository.Region,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			cfg.Logger.Panic("failed to create aws session", zap.Error(err))
		}
		ts.ecrAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(cfg.Repository.Region))
	}
	return ts
}

type tester struct {
	cfg    *Config
	ecrAPI ecriface.ECRAPI
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func EnvRepository() string {
	return Env() + "_REPOSITORY"
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	img, err := ts.checkECRImage()
	if err != nil {
		return err
	}

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createDeployment(img); err != nil {
		return err
	}

	if err := ts.checkDeployment(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() (err error) {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

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

const (
	deploymentName = "php-apache-deployment"
	appName        = "php-apache"
	appImageName   = "pjlewis/php-apache"
)

func (ts *tester) checkECRImage() (img string, err error) {
	// check ECR permission
	// ref. https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/jobs-echo/jobs-echo.go#L75-L90
	img, _, err = ts.cfg.Repository.Describe(ts.cfg.Logger, ts.ecrAPI)
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ECR image", zap.Error(err))
		img = appImageName
	}
	return img, nil
}

func (ts *tester) createDeployment(containerImg string) error {
	var nodeSelector map[string]string
	if len(ts.cfg.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating PHP Apache Deployment", zap.Any("node-selector", nodeSelector))
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
									Image:           containerImg,
									ImagePullPolicy: core_v1.PullAlways,
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
			ts.cfg.Logger.Info("PHP Apache Deployment already exists")
			return nil
		}
		return fmt.Errorf("failed to create PHP Apache Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created PHP Apache Deployment")
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
		}),
	)
	cancel()
	return err
}
