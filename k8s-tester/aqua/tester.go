// package aqua installs aqua helm charts.
// ref https://github.com/aquasecurity/aqua-helm
package aqua

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
	"github.com/aws/aws-k8s-tester/k8s-tester/helm"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
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
	// HelmChartRepoURL is the helm chart repo URL.
	HelmChartRepoURL string `json:"helm_chart_repo_url"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
	// AquaLicense is the license used from the suceess center for Kubenenforcer
	AquaLicense string `json:"aqua_license"`
	// AquaUsername is the username for the suceess center used to pull images
	AquaUsername string `json:"aqua_username"`
	// AquaUsername is the password for the suceess center used to pull images
	AquaPassword string `json:"aqua_password"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}
	if cfg.AquaUsername == "" {
		return errors.New("empty Aqua Usernmae")
	}
	if cfg.AquaPassword == "" {
		return errors.New("empty Aqua Password")
	}
	return nil
}

const chartName = "server"

const (
	DefaultMinimumNodes     int = 1
	chartRepoName               = "aqua"
	chartRepoURL                = "https://helm.aquasec.com"
	DefaultHelmChartRepoURL     = "https://github.com/aquasecurity/aqua-helm/archive/refs/tags/6.0.2.tar.gz"
)

func NewDefault() *Config {
	return &Config{
		Enable:           false,
		Prompt:           false,
		MinimumNodes:     DefaultMinimumNodes,
		HelmChartRepoURL: DefaultHelmChartRepoURL,
		Namespace:        pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
	}
}

func New(cfg *Config) k8s_tester.Tester {
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

	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}
	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}
	if err := ts.checkForStorageClass(); err != nil {
		return err
	}
	if err := helm.AddUpdate(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelmAqua(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := ts.deleteHelmAqua(); err != nil {
		errs = append(errs, err.Error())
	}

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

func (ts *tester) checkForStorageClass() (err error) {
	storageclass, err := client.ListStorageClass(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		5,
		5*time.Second,
	)
	for _, class := range storageclass {
		if class.ObjectMeta.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			ts.cfg.Logger.Info("found default STORAGE CLASS, proceeding to tests")
			return nil
		} else {
			return errors.New("No Default StroageClass")
		}
	}
	return nil
}

// https://github.com/aquasecurity/aqua-helm/tree/6.2/aqua-quickstart
func (ts *tester) createHelmAqua() error {
	values := map[string]interface{}{
		// "ke": map[string]interface{}{
		// 	"aquasecret": map[string]interface{}{
		// 		"kubeEnforcerToken": ts.cfg.AquaLicense,
		// 	},
		// },
		"platform": "k8s",
		"imageCredentials": map[string]interface{}{
			"username": ts.cfg.AquaUsername,
			"password": ts.cfg.AquaPassword,
		},
		"web": map[string]interface{}{
			"service": map[string]interface{}{
				"type": "ClusterIP",
			},
		},
	}
	getAllArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"get",
		"all",
	}
	getAllCmd := strings.Join(getAllArgs, " ")

	descArgsDs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"describe",
		"deployment.apps/aqua-console",
	}
	descCmdDs := strings.Join(descArgsDs, " ")

	descArgsPods := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"describe",
		"pods",
		"--selector=app=aqua-console",
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	logArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"logs",
		"--selector=app=aqua-console",
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        10 * time.Minute,
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
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed", zap.Error(err))
				ts.cfg.Logger.Warn("Aqua Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsDs[0], descArgsDs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe daemonset' failed", zap.Error(err))
				ts.cfg.Logger.Warn("Aqua Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdDs, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe pods' failed", zap.Error(err))
				ts.cfg.Logger.Warn("Aqua Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdPods, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", logsCmd, out)
		},
		QueryInterval: 30 * time.Second,
	})
}

func (ts *tester) deleteHelmAqua() error {
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
