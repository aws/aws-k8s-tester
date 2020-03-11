package appmesh

import (
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"os"
	"strings"
	"time"
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
	if err := ts.createNamespace(); err != nil {
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
	if err := ts.deleteNamespace(); err != nil {
		errs = append(errs, err.Error())
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

func (ts *tester) createNamespace() error {
	ts.cfg.Logger.Info("creating namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnAppMesh.Namespace))
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
				Labels: map[string]string{
					"name": ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnAppMesh.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteNamespace() error {
	ts.cfg.Logger.Info("deleting namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnAppMesh.Namespace))
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil {
		// ref. https://github.com/aws/aws-k8s-tester/issues/79
		if !strings.Contains(err.Error(), ` not found`) {
			return err
		}
	}
	ts.cfg.Logger.Info("deleted namespace", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}
