// Package k8s_tester implements k8s-tester.
// Same run order as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617.
package k8s_tester

import (
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/client"
	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
	fluent_bit "github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit"
	jobs_echo "github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo"
	jobs_pi "github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi"
	kubernetes_dashboard "github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard"
	metrics_server "github.com/aws/aws-k8s-tester/k8s-tester/metrics-server"
	nlb_hello_world "github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	k8s_client "k8s.io/client-go/kubernetes"
)

// Config defines k8s-tester configurations.
// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
// By default, it uses the environmental variables as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
// TODO: support https://github.com/onsi/ginkgo.
type Config struct {
	EnablePrompt bool `json:"-"`

	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Stopc     chan struct{} `json:"-"`

	ClientConfig *client.Config `json:"-"`

	ClusterName string `json:"cluster-name"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum-nodes"`

	CloudWatchAgent     *cloudwatch_agent.Config     `json:"add-on-cloudwatch-agent"`
	MetricsServer       *metrics_server.Config       `json:"add-on-metrics-server"`
	FluentBit           *fluent_bit.Config           `json:"add-on-fluent-bit"`
	KubernetesDashboard *kubernetes_dashboard.Config `json:"add-on-kubernetes-dashboard"`

	NLBHelloWorld *nlb_hello_world.Config `json:"add-on-nlb-hellow-world"`

	JobsPi       *jobs_pi.Config   `json:"add-on-jobs-pi"`
	JobsEcho     *jobs_echo.Config `json:"add-on-jobs-echo"`
	CronJobsEcho *jobs_echo.Config `json:"add-on-cron-jobs-echo"`
}

const DefaultMinimumNodes = 1

func New(cfg Config) k8s_tester.Tester {
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
	cfg Config
	cli k8s_client.Interface
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cli); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.EnablePrompt {
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
