// Package falcon install Falcon Operator
package falcon

import (
	"context"
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
	"go.uber.org/zap"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`
}

func (cfg *Config) ValidateAndSetDefaults() error {

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

	return fmt.Errorf("NOT IMPLEMENTED")
}

func (ts *tester) Delete() error {
	ctx := context.TODO()

	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	if err := ts.deleteOperator(ctx); err != nil {
		return err
	}

	return fmt.Errorf("NOT IMPLEMENTED")
}

func (ts *tester) deployOperator(ctx context.Context) error {
	ts.cfg.Logger.Info("deploying: ", zap.String("Operator", "falcon-operator"))

	return ts.kubectl(ctx, "apply", operatorSpecUri, time.Minute)
}

func (ts tester) deleteOperator(ctx context.Context) error {
	ts.cfg.Logger.Info("uninstalling: ", zap.String("Operator", "falcon-operator"))

	return ts.kubectl(ctx, "delete", operatorSpecUri, time.Minute)
}

func (ts *tester) waitForOperatorRunning(ctx context.Context) error {
	return ts.waitForDeployment(ctx, operatorNamespace, operatorDeployment, time.Second*180)
}

const (
	operatorNamespace  string = "falcon-operator"
	operatorDeployment string = "falcon-operator-controller-manager"
	operatorSpecUri    string = "https://raw.githubusercontent.com/CrowdStrike/falcon-operator/main/deploy/falcon-operator.yaml"
)

func operatorSpecData() ([]byte, error) {
	resp, err := http.Get(operatorSpecUri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
