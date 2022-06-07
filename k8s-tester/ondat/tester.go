// package ondat installs ondat agent.
// ref. https://ondat.io
package ondat

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
	// OndatNamespace for the Ondat cluster
	OndatNamespace string `json:"ondat_namespace"`
	// EtcdNamespace for the etcd cluster
	EtcdNamespace string `json:"etcd_namespace"`
	// EtcdReplicas to be used for etcd
	EtcdReplicas int `json:"etcd_replicas"`
	// EtcdStorage is the amount of GiB to allocate to each etcd node
	EtcdStorage int `json:"etcd_storage"`
	// EtcdStorageClass to be used for etcd
	EtcdStorageClass string `json:"etcd_storageclass"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}
	if cfg.OndatNamespace == "" {
		cfg.OndatNamespace = DefaultOndatNamespace
	}
	if cfg.EtcdNamespace == "" {
		cfg.EtcdNamespace = DefaultEtcdNamespace
	}
	if cfg.EtcdReplicas == 0 {
		cfg.EtcdReplicas = DefaultEtcdReplicas
	}
	if cfg.EtcdStorage == 0 {
		cfg.EtcdStorage = DefaultEtcdStorage
	}
	if cfg.EtcdStorageClass == "" {
		return errors.New("empty etcd StorageClass")
	}
	return nil
}

const chartName = "ondat"

const (
	DefaultMinimumNodes     int = 1
	DefaultOndatNamespace       = "storageos"
	DefaultEtcdNamespace        = "storageos-etcd"
	DefaultEtcdStorage      int = 12
	DefaultEtcdReplicas     int = 1
	DefaultHelmChartRepoURL     = "https://github.com/ondat/charts/releases/download/ondat-0.0.1/ondat-0.0.1.tgz"
)

func NewDefault() *Config {
	return &Config{
		Enable:           false,
		Prompt:           false,
		MinimumNodes:     DefaultMinimumNodes,
		EtcdReplicas:     DefaultEtcdReplicas,
		EtcdStorage:      DefaultEtcdStorage,
		OndatNamespace:   DefaultOndatNamespace,
		EtcdNamespace:    DefaultEtcdNamespace,
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
	if err := ts.createHelmondat(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := ts.deleteHelmondat(); err != nil {
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

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.OndatNamespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.EtcdNamespace,
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

// https://github.com/ondat/charts/blob/main/charts/ondat/values.yaml
func (ts *tester) createHelmondat() error {
	values := map[string]interface{}{
		"ondat": map[string]interface{}{
			"namespace": ts.cfg.OndatNamespace,
		},
		"ondat-operator": map[string]interface{}{
			"cluster": map[string]interface{}{
				"namespace": ts.cfg.OndatNamespace,
			},
		},
		"etcd-cluster-operator": map[string]interface{}{
			"cluster": map[string]interface{}{
				"namespace":    ts.cfg.EtcdNamespace,
				"replicas":     ts.cfg.EtcdReplicas,
				"storage":      fmt.Sprintf("%dGi", ts.cfg.EtcdStorage),
				"storageclass": ts.cfg.EtcdStorageClass,
			},
		},
	}

	getAllOpArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"get",
		"all",
	}
	getAllOpCmd := strings.Join(getAllOpArgs, " ")

	getAllArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.OndatNamespace,
		"get",
		"all",
	}
	getAllCmd := strings.Join(getAllArgs, " ")

	getAllEtcdArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.EtcdNamespace,
		"get",
		"all",
	}
	getAllEtcdCmd := strings.Join(getAllEtcdArgs, " ")

	descArgsDs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.OndatNamespace,
		"describe",
		"daemonset.apps/storageos-node",
	}
	descCmdDs := strings.Join(descArgsDs, " ")

	descArgsPods := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.OndatNamespace,
		"describe",
		"pods",
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	descArgsOpPods := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"describe",
		"pods",
	}
	descCmdOpPods := strings.Join(descArgsOpPods, " ")

	descArgsEtcdPods := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.EtcdNamespace,
		"describe",
		"pods",
	}
	descCmdEtcdPods := strings.Join(descArgsEtcdPods, " ")

	logOpArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"logs",
		"--all-containers=true",
		"--timestamps",
	}
	logsOpCmd := strings.Join(logOpArgs, " ")

	logArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.OndatNamespace,
		"logs",
		"--selector=app=storageos",
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	logEtcdArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.EtcdNamespace,
		"logs",
		"--selector=app.kubernetes.io/name=etcd",
		"--all-containers=true",
		"--timestamps",
	}
	logsEtcdCmd := strings.Join(logEtcdArgs, " ")

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        10 * time.Minute,
		KubeconfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      ts.cfg.Namespace,
		ChartRepoURL:   ts.cfg.HelmChartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
		LogFunc: func(format string, v ...interface{}) {
			ts.cfg.Logger.Info(fmt.Sprintf("[install] "+format, v...))
		},
		QueryFunc: func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllOpArgs[0], getAllOpArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed for Operator", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllOpCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed for Ondat", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, getAllEtcdArgs[0], getAllEtcdArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed for Etcd", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllEtcdCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsDs[0], descArgsDs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe daemonset' failed for Ondat", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdDs, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsOpPods[0], descArgsOpPods[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe pods' failed for Operator", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdOpPods, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe pods' failed for Ondat", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdPods, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsEtcdPods[0], descArgsEtcdPods[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe pods' failed for Etcd", zap.Error(err))
				ts.cfg.Logger.Warn("ondat Tests::", zap.String("TEST", "FAILED"))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdEtcdPods, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logOpArgs[0], logOpArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed for Operator", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", logsOpCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed for Ondat", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", logsCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, logEtcdArgs[0], logEtcdArgs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl logs' failed for Etcd", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", logsEtcdCmd, out)
		},
		QueryInterval: 30 * time.Second,
	})
}

func (ts *tester) deleteHelmondat() error {
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
