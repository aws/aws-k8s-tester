// Package wordpress installs wordpress.
// Replace https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/wordpress
package wordpress

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
	"github.com/aws/aws-k8s-tester/k8s-tester/helm"
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

	UserName string `json:"user_name"`
	Password string `json:"password"`

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
	chartRepoName = "bitnami"
	chartRepoURL  = "https://charts.bitnami.com/bitnami"
	chartName     = "wordpress"
	serviceName   = "wordpress"
)

const (
	DefaultMinimumNodes int = 1
	DefaultUserName         = "foo"
	DefaultPassword         = "bar"
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       false,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
		UserName:     DefaultUserName,
		Password:     DefaultPassword,
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

	if err := helm.AddUpdate(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelm(); err != nil {
		return err
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

	if ts.cfg.ELBARN != "" {
		if err := aws_v1_elb.DeleteELBv2(
			ts.cfg.Logger,
			ts.cfg.ELB2API,
			ts.cfg.ELBARN,
		); err != nil {
			errs = append(errs, fmt.Sprintf("failed to delete ELB (%v)", err))
		}
	}

	if err := ts.deleteHelm(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources, should we continue?", action)
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

func (ts *tester) createHelm() error {
	// https://github.com/helm/charts/blob/master/stable/wordpress/values.yaml
	values := map[string]interface{}{
		"wordpressUsername": ts.cfg.UserName,
		"wordpressPassword": ts.cfg.Password,
		"persistence": map[string]interface{}{
			"enabled": true,
			// use CSI driver with volume type "gp2", as in launch configuration
			"storageClassName": "gp2",
		},
		// https://github.com/helm/charts/blob/master/stable/mariadb/values.yaml
		"mariadb": map[string]interface{}{
			"enabled": true,
			"rootUser": map[string]interface{}{
				"password":      ts.cfg.Password,
				"forcePassword": false,
			},
			"db": map[string]interface{}{
				"name":     "wordpress",
				"user":     ts.cfg.UserName,
				"password": ts.cfg.Password,
			},
			"master": map[string]interface{}{
				"persistence": map[string]interface{}{
					"enabled": true,
					// use CSI driver with volume type "gp2", as in launch configuration
					"storageClassName": "gp2",
				},
			},
		},
	}

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        15 * time.Minute,
		KubeconfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      ts.cfg.Namespace,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
		LogFunc: func(format string, v ...interface{}) {
			ts.cfg.Logger.Info(fmt.Sprintf("[install] "+format, v...))
		},
		QueryFunc: func() {
			getAllArgs := []string{
				ts.cfg.Client.Config().KubectlPath,
				"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
				"--namespace=" + ts.cfg.Namespace,
				"get",
				"all",
			}
			getAllCmd := strings.Join(getAllArgs, " ")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)
		},
		QueryInterval: 30 * time.Second,
	})
}

func (ts *tester) deleteHelm() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Timeout:        15 * time.Minute,
		KubeconfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      ts.cfg.Namespace,
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
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

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB wordpress ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB wordpress name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB wordpress URL: %s\n\n", elbURL)

	ts.cfg.Logger.Info("waiting before testing wordpress Service")
	time.Sleep(20 * time.Second)

	htmlChecked := false
	retryStart := time.Now()
	for time.Since(retryStart) < 3*time.Minute {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("wordpress Service creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := http.ReadInsecure(ts.cfg.Logger, ioutil.Discard, elbURL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read NLB wordpress Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Fprintf(ts.cfg.LogWriter, "\nNLB wordpress Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<p>Welcome to WordPress. This is your first post`) {
			ts.cfg.Logger.Info("read wordpress Service; exiting", zap.String("host-name", hostName))
			htmlChecked = true
			break
		}

		ts.cfg.Logger.Warn("unexpected wordpress Service output; retrying")
	}

	fmt.Fprintf(ts.cfg.LogWriter, "\nNLB wordpress ARN: %s\n", elbARN)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB wordpress name: %s\n", elbName)
	fmt.Fprintf(ts.cfg.LogWriter, "NLB wordpress URL: %s\n", elbURL)
	fmt.Fprintf(ts.cfg.LogWriter, "WordPress UserName: %s\n", ts.cfg.UserName)
	fmt.Fprintf(ts.cfg.LogWriter, "WordPress Password: %d characters\n\n", len(ts.cfg.Password))

	if !htmlChecked {
		return fmt.Errorf("NLB wordpress %q did not return expected HTML output", elbURL)
	}

	return nil
}
