// Package falcon install Falcon Operator
package falcon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/file"
	falconv1alpha1 "github.com/crowdstrike/falcon-operator/apis/falcon/v1alpha1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	FalconClientId     string `json:"falcon_client_id"`
	FalconClientSecret string `json:"falcon_client_secret"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.FalconClientId == "" || cfg.FalconClientSecret == "" {
		return errors.New("CrowdStrike Falcon API credentials missing")
	}

	return nil
}

func NewDefault() *Config {
	return &Config{
		Enable: false,
		Prompt: false,
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
	ctx := context.TODO()

	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}
	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); err != nil || len(nodes) == 0 {
		return fmt.Errorf("failed to validate minimum nodes requirement (nodes %v, error %v)", len(nodes), err)

	}
	if err := ts.deployOperator(ctx); err != nil {
		return err
	}
	if err := ts.waitForOperatorRunning(ctx); err != nil {
		return err
	}
	if err := ts.deployFalconContainer(ctx); err != nil {
		return err
	}

	return fmt.Errorf("NOT IMPLEMENTED")
}

func (ts *tester) Delete() error {
	ctx := context.TODO()

	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	if err := ts.deleteFalconContainer(ctx); err != nil {
		return err
	}
	return ts.deleteOperator(ctx)
}

func (ts *tester) deployOperator(ctx context.Context) error {
	ts.cfg.Logger.Info("deploying: ", zap.String("Operator", "falcon-operator"))

	return ts.kubectlFile(ctx, "apply", operatorSpecUri, time.Minute)
}

func (ts tester) deleteOperator(ctx context.Context) error {
	ts.cfg.Logger.Info("uninstalling: ", zap.String("Operator", "falcon-operator"))

	if exists, err := ts.deploymentExists(ctx, operatorNamespace, operatorDeployment); err != nil || !exists {
		return err
	}
	return ts.kubectlFile(ctx, "delete", operatorSpecUri, time.Minute*2)
}

func (ts *tester) waitForOperatorRunning(ctx context.Context) error {
	return ts.waitForDeployment(ctx, operatorNamespace, operatorDeployment, time.Second*180)
}

func (ts *tester) deployFalconContainer(ctx context.Context) error {
	container := falconv1alpha1.FalconContainer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "FalconContainer",
			APIVersion: falconv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Spec: falconv1alpha1.FalconContainerSpec{
			FalconAPI: falconv1alpha1.FalconAPI{
				CloudRegion:  "autodiscover",
				ClientId:     ts.cfg.FalconClientId,
				ClientSecret: ts.cfg.FalconClientSecret,
			},
			Registry: falconv1alpha1.RegistrySpec{
				Type: falconv1alpha1.RegistryTypeCrowdStrike,
			},
			InstallerArgs: []string{
				"-disable-default-ns-injection",
			},
		},
	}

	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	if err := enc.Encode(container); err != nil {
		return err
	}

	fpath, err := file.WriteTempFile(out.Bytes())
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("applying FalconContainer YAML", zap.String("path", fpath))
	return ts.kubectlFile(ctx, "apply", fpath, time.Minute*2)
}

func (ts *tester) deleteFalconContainer(ctx context.Context) error {
	ts.cfg.Logger.Info("uninstalling: ", zap.String("product", "falcon-container"))
	if exists, err := ts.deploymentExists(ctx, injectorNamespace, injectorDeployment); err != nil || !exists {
		return err
	}

	return ts.kubectl(ctx, []string{"delete", "falconcontainers.falcon.crowdstrike.com", "--all"}, time.Minute)
}

const (
	operatorNamespace  string = "falcon-operator"
	operatorDeployment string = "falcon-operator-controller-manager"
	operatorSpecUri    string = "https://raw.githubusercontent.com/CrowdStrike/falcon-operator/main/deploy/falcon-operator.yaml"
	injectorNamespace  string = "falcon-system"
	injectorDeployment string = "injector"
)

func operatorSpecData() ([]byte, error) {
	resp, err := http.Get(operatorSpecUri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
