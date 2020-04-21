// Package csiebs installs "aws-ebs-csi-driver".
// ref. https://github.com/kubernetes-sigs/aws-ebs-csi-driver
// ref. https://github.com/kubernetes-sigs/aws-ebs-csi-driver/blob/master/aws-ebs-csi-driver/values.yaml
package csiebs

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/helm"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
)

// Config defines AWS EBS CSI Driver configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines AWS EBS CSI Driver tester
type Tester interface {
	// Create installs AWS EBS CSI Driver.
	Create() error
	// Delete deletes AWS EBS CSI Driver.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

const chartName = "aws-ebs-csi-driver"

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnCSIEBS.Created {
		ts.cfg.Logger.Info("skipping create AddOnCSIEBS")
		return nil
	}

	ts.cfg.EKSConfig.AddOnCSIEBS.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnCSIEBS.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnCSIEBS.CreateTookString = ts.cfg.EKSConfig.AddOnCSIEBS.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createHelmCSI(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnCSIEBS.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCSIEBS")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnCSIEBS.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnCSIEBS.DeleteTookString = ts.cfg.EKSConfig.AddOnCSIEBS.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmCSI(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCSIEBS.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/helm/charts/blob/master/stable/wordpress/values.yaml
// https://github.com/helm/charts/blob/master/stable/mariadb/values.yaml
func (ts *tester) createHelmCSI() error {
	values := make(map[string]interface{})
	values["enableVolumeScheduling"] = true
	values["enableVolumeResizing"] = true
	values["enableVolumeSnapshot"] = true

	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        10 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      "kube-system",
		ChartRepoURL:   ts.cfg.EKSConfig.AddOnCSIEBS.ChartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
	})
}

func (ts *tester) deleteHelmCSI() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        10 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      "kube-system",
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
}
