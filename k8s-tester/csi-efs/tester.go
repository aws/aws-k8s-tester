// Package csi_efs tests the CSI drivers storage capabilities
package csi_efs

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
	helm "github.com/aws/aws-k8s-tester/k8s-tester/helm"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storage_v1 "k8s.io/api/storage/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	api_resource "k8s.io/apimachinery/pkg/api/resource"
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

	// HelmChartRepoURL is the helm chart repo URL.
	HelmChartRepoURL string `json:"helm_chart_repo_url"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}
	if cfg.HelmChartRepoURL == "" {
		cfg.HelmChartRepoURL = DefaultHelmChartRepoURL
	}
	return nil
}

const DefaultHelmChartRepoURL string = "https://kubernetes-sigs.github.io/aws-efs-csi-driver"

const DefaultMinimumNodes int = 1

const (
	chartName           string = "aws-efs-csi-driver"
	storageClassName    string = "efs-sc"
	pvcName             string = "efs-claim"
	provisioner         string = "efs.csi.aws.com"
	VolumeBindingMode   string = "WaitForFirstConsumer"
	provisionPodName    string = "provisionpod"
	provisionVolumeName string = "provisionvolume"
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       true,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    "kube-system",
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

var values = map[string]interface{}{}

var graceperiod = int64(0)

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
	if err := ts.installChart(); err != nil {
		return err
	}
	if err := ts.createSC(); err != nil {
		return err
	}
	if err := ts.createPVC(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	var errs []string
	if err := ts.deletePVC(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete helm chart EBS (%v)", err))
	}
	if err := ts.deleteSC(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete helm chart EBS (%v)", err))
	}
	if err := ts.deleteChart(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete helm chart EBS (%v)", err))
	}
	if ts.cfg.Namespace != "kube-system" {
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

func (ts *tester) installChart() error {
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
		"daemonset.apps/efs-csi-node",
	}
	descCmdDs := strings.Join(descArgsDs, " ")

	descArgsPods := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"describe",
		"pods",
		"--selector=app=efs-csi-controller",
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	logArgs := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		"--namespace=" + ts.cfg.Namespace,
		"logs",
		"--selector=app=efs-csi-controller",
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
		ChartRepoURL:   ts.cfg.HelmChartRepoURL,
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
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsDs[0], descArgsDs[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe daemonset' failed", zap.Error(err))
			}
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmdDs, out)

			ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
			cancel()
			out = strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe pods' failed", zap.Error(err))
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

func (ts *tester) deleteChart() error {
	ts.cfg.Logger.Info("deleting %s: %s", zap.String("helm-chart-name", chartName))
	err := helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Timeout:        3 * time.Minute,
		KubeconfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      "kube-system",
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
	if err == nil {
		ts.cfg.Logger.Info("deleted helm chart", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName))
		return nil
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
		ts.cfg.Logger.Info("helm chart already deleted", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName), zap.Error(err))
		return nil
	}
	ts.cfg.Logger.Warn("failed to delete helm chart", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName), zap.Error(err))
	return err
}

func (ts *tester) createSC() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	firstConsumerBinding := storage_v1.VolumeBindingWaitForFirstConsumer
	allowVolumeExpansion := true
	_, err = ts.cfg.Client.KubernetesClient().StorageV1().StorageClasses().Create(
		ctx,
		&storage_v1.StorageClass{
			TypeMeta: meta_v1.TypeMeta{
				APIVersion: "storage.k8s.io/v1",
				Kind:       "StorageClass",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          provisioner,
			AllowVolumeExpansion: &allowVolumeExpansion,
			VolumeBindingMode:    &firstConsumerBinding,
			Parameters: map[string]string{
				"basePath": "/dynamic_provisioning",
			},
		},
		meta_v1.CreateOptions{},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create StorageClasses (%v)", err)
	}
	ts.cfg.Logger.Info("created a StorageClasses for EFS")
	return nil
}

func (ts *tester) deleteSC() (err error) {
	ts.cfg.Logger.Info("deleting storageClass for EFS")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	err = ts.cfg.Client.KubernetesClient().StorageV1().StorageClasses().Delete(
		ctx,
		storageClassName,
		meta_v1.DeleteOptions{
			PropagationPolicy: &foreground,
		},
	)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("delete StorageClasses for EFS")
	return nil
}

func (ts *tester) createPVC() error {
	scName := storageClassName
	ts.cfg.Logger.Info("creating PersistentVolumeClaim for EFS, provisioning test")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Create(
		ctx,
		&core_v1.PersistentVolumeClaim{
			TypeMeta: meta_v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name: pvcName,
			},
			Spec: core_v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				StorageClassName: &scName,
				Resources: core_v1.ResourceRequirements{
					Requests: core_v1.ResourceList{
						core_v1.ResourceStorage: api_resource.MustParse("5Gi"),
					},
				},
			},
		},
		meta_v1.CreateOptions{},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaims (%v)", err)
	}
	ts.cfg.Logger.Info("created a PersistentVolumeClaims for EFS")
	return nil
}

var foreground = meta_v1.DeletePropagationForeground

func (ts *tester) deletePVC() error {
	ts.cfg.Logger.Info("deleting PersistentVolumeClaim for EFS, Provisioning test")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	err := ts.cfg.Client.KubernetesClient().CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Delete(
		ctx,
		pvcName,
		meta_v1.DeleteOptions{
			PropagationPolicy: &foreground,
		},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete PersistentVolumeClaim (%v)", err)
	}
	ts.cfg.Logger.Info("Deleted a PersistentVolumeClaim for EFS")
	return nil
}
