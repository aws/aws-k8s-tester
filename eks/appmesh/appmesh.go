// Package appmesh implements App Mesh add-on.
package appmesh

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8sclient "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines AppMesh configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	CFNAPI    cloudformationiface.CloudFormationAPI
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester defines AppMesh tester
type Tester interface {
	// Installs AppMesh controller/injector
	Create() error

	// Clean up AppMesh controller/injector
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{
		cfg: cfg,
	}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping create AddOnAppMesh")
		return nil
	}

	ts.cfg.EKSConfig.AddOnAppMesh.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnAppMesh.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnAppMesh.CreateTookString = ts.cfg.EKSConfig.AddOnAppMesh.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createAppMeshAddOnCFNStack(); err != nil {
		return err
	}
	if err := k8sclient.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnAppMesh.Namespace); err != nil {
		return err
	}
	if err := ts.installController(); err != nil {
		return err
	}
	if err := ts.installInjector(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping delete AddOnAppMesh")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnAppMesh.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnAppMesh.DeleteTookString = ts.cfg.EKSConfig.AddOnAppMesh.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.uninstallInjector(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.uninstallController(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8sclient.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		k8sclient.DefaultNamespaceDeletionInterval,
		k8sclient.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete AppMesh namespace (%v)", err))
	}

	if err := ts.deleteAppMeshAddOnCFNStack(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnAppMesh.Created = false
	return ts.cfg.EKSConfig.Sync()
}
