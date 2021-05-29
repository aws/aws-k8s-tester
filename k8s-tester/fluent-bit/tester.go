// Package logger_tests installs a simple "Hello World" application with a logger and tests the logger function.
package fluent_bit

import (
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	k8s_client "k8s.io/client-go/kubernetes"
)

type Config struct {
	EnablePrompt bool `json:"-"`

	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Stopc     chan struct{} `json:"-"`

	ClientConfig *client.Config `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum-nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
}

const DefaultMinimumNodes int = 1

func NewDefault() *Config {
	return &Config{
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	ccfg, err := client.CreateConfig(cfg.ClientConfig)
	if err != nil {
		cfg.Logger.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		cfg.Logger.Panic("failed to create client", zap.Error(err))
	}

	return &tester{
		cfg: cfg,
		cli: cli,
	}
}

type tester struct {
	cfg *Config
	cli k8s_client.Interface
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cli); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := client.CreateNamespace(ts.cfg.Logger, ts.cli, ts.cfg.Namespace); err != nil {
		return err
	}

	if err := ts.createServiceAccount(); err != nil {
		return err
	}

	if err := ts.createRBACClusterRole(); err != nil {
		return err
	}

	if err := ts.createRBACClusterRoleBinding(); err != nil {
		return err
	}

	if err := ts.createAppConfigMap(); err != nil {
		return err
	}

	if err := ts.createDaemonSet(); err != nil {
		return err
	}

	if err := ts.checkDaemonSet(); err != nil {
		return err
	}

	if err := ts.createService(); err != nil {
		return err
	}

	if err := ts.testHTTPClient(); err != nil {
		return err
	}

	if err := ts.testLogsWithinNamespace(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := client.DeleteServiceAccount(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appServiceAccountName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ServiceAccount (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting ServiceAccount")

	if err := client.DeleteRBACRole(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appRBACRoleName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Role (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("Role", appName))

	if err := client.DeleteRBACRoleBinding(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appRBACRoleBindingName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete RoleBinding (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("RoleBinding", appName))

	if err := client.DeleteConfigmap(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appConfigMapNameConfig,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Configmap (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("Configmap", appName))

	if err := client.DeleteDaemonSet(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete DaemonSet (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("DaemonSet", appName))
	ts.cfg.Logger.Info("wait for a minute after deleting DaemonSet")
	time.Sleep(time.Minute)

	if err := client.DeleteService(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		appName,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Service (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("Service", appName))

	if err := client.DeletePod(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		containerHTTPClient,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Pod (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("Pod", containerHTTPClient))

	if err := client.DeletePod(
		ts.cfg.Logger,
		ts.cli,
		ts.cfg.Namespace,
		loggingPod,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Pod (%v)", err))
	}
	ts.cfg.Logger.Info("Deleting %s: %s", zap.String("Pod", loggingPod))

	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cli,
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
	if ts.cfg.EnablePrompt {
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
