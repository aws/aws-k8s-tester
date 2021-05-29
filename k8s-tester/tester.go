// Package k8s_tester implements k8s-tester.
// Same run order as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617.
package k8s_tester

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	"github.com/aws/aws-k8s-tester/k8s-tester/version"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-k8s-tester/utils/http"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	k8s_client "k8s.io/client-go/kubernetes"
)

func New(cfg *Config) k8s_tester.Tester {
	lg, logWriter, _, err := log.NewWithStderrWriter(cfg.LogLevel, cfg.LogOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	fmt.Fprintf(logWriter, cfg.Colorize("\n\n\n[yellow]*********************************\n"))
	fmt.Fprintln(logWriter, "😎 🙏 🚶 ✔️ 👍")
	fmt.Fprintf(logWriter, cfg.Colorize("[light_green]New k8s-tester %q [default](%q)\n\n"), cfg.ConfigPath, version.Version())

	lg.Info("mkdir", zap.String("kubectl-path-dir", filepath.Dir(cfg.KubectlPath)))
	if err = os.MkdirAll(filepath.Dir(cfg.KubectlPath), 0700); err != nil {
		lg.Panic("could not create", zap.String("dir", filepath.Dir(cfg.KubectlPath)), zap.Error(err))
	}
	if !file.Exist(cfg.KubectlPath) {
		if cfg.KubectlDownloadURL == "" {
			lg.Panic("kubectl does not exist, kubectl download URL empty", zap.String("kubectl-path", cfg.KubectlPath))
		}
		cfg.KubectlPath, _ = filepath.Abs(cfg.KubectlPath)
		lg.Info("downloading kubectl", zap.String("kubectl-path", cfg.KubectlPath))
		if err = http.Download(lg, os.Stderr, cfg.KubectlDownloadURL, cfg.KubectlPath); err != nil {
			lg.Panic("failed to download kubectl", zap.Error(err))
		}
	} else {
		lg.Info("skipping kubectl download; already exist", zap.String("kubectl-path", cfg.KubectlPath))
	}
	if err = file.EnsureExecutable(cfg.KubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
		err = nil
	}

	ts := &tester{
		cfg:       cfg,
		logger:    lg,
		logWriter: logWriter,
		clientConfig: &client.Config{
			Logger:            lg,
			KubectlPath:       cfg.KubectlPath,
			KubeconfigPath:    cfg.KubeconfigPath,
			KubeconfigContext: cfg.KubeconfigContext,
		},
		testers: make([]k8s_tester.Tester, 0),
	}

	ccfg, err := client.CreateConfig(ts.clientConfig)
	if err != nil {
		lg.Panic("failed to create client config", zap.Error(err))
	}
	ts.cli, err = k8s_client.NewForConfig(ccfg)
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
	if cfg.AddOnCloudwatchAgent != nil && cfg.AddOnCloudwatchAgent.Enable {
		cfg.AddOnCloudwatchAgent.ClientConfig = ts.clientConfig
		cfg.AddOnCloudwatchAgent.Client = ts.cli
		ts.testers = append(ts.testers, cloudwatch_agent.New(cfg.AddOnCloudwatchAgent))
	}
	if cfg.AddOnMetricsServer != nil && cfg.AddOnMetricsServer.Enable {
		cfg.AddOnMetricsServer.ClientConfig = ts.clientConfig
		cfg.AddOnMetricsServer.Client = ts.cli
		ts.testers = append(ts.testers, metrics_server.New(cfg.AddOnMetricsServer))
	}
	if cfg.AddOnFluentBit != nil && cfg.AddOnFluentBit.Enable {
		cfg.AddOnFluentBit.ClientConfig = ts.clientConfig
		cfg.AddOnFluentBit.Client = ts.cli
		ts.testers = append(ts.testers, fluent_bit.New(cfg.AddOnFluentBit))
	}
	if cfg.AddOnKubernetesDashboard != nil && cfg.AddOnKubernetesDashboard.Enable {
		cfg.AddOnKubernetesDashboard.ClientConfig = ts.clientConfig
		cfg.AddOnKubernetesDashboard.Client = ts.cli
		ts.testers = append(ts.testers, kubernetes_dashboard.New(cfg.AddOnKubernetesDashboard))
	}
	if cfg.AddOnNLBHelloWorld != nil && cfg.AddOnNLBHelloWorld.Enable {
		cfg.AddOnNLBHelloWorld.ClientConfig = ts.clientConfig
		cfg.AddOnNLBHelloWorld.Client = ts.cli
		ts.testers = append(ts.testers, nlb_hello_world.New(cfg.AddOnNLBHelloWorld))
	}
	if cfg.AddOnJobsPi != nil && cfg.AddOnJobsPi.Enable {
		cfg.AddOnJobsPi.ClientConfig = ts.clientConfig
		cfg.AddOnJobsPi.Client = ts.cli
		ts.testers = append(ts.testers, jobs_pi.New(cfg.AddOnJobsPi))
	}
	if cfg.AddOnJobsEcho != nil && cfg.AddOnJobsEcho.Enable {
		cfg.AddOnJobsEcho.ClientConfig = ts.clientConfig
		cfg.AddOnJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(cfg.AddOnJobsEcho))
	}
	if cfg.AddOnCronJobsEcho != nil && cfg.AddOnCronJobsEcho.Enable {
		cfg.AddOnCronJobsEcho.ClientConfig = ts.clientConfig
		cfg.AddOnCronJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(cfg.AddOnCronJobsEcho))
	}

	return ts
}

type tester struct {
	cfg *Config

	logger       *zap.Logger
	logWriter    io.Writer
	clientConfig *client.Config
	cli          k8s_client.Interface

	// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
	testers []k8s_tester.Tester
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return true }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	nodes, err := client.ListNodes(ts.cli)
	if len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}
	ts.cfg.TotalNodes = len(nodes)
	ts.cfg.Sync()

	// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
	for idx, tt := range ts.testers {
		_ = idx
		if !tt.Enabled() {
			continue
		}
		if err := tt.Apply(); err != nil {
			return err
		}
	}

	return nil
}

// 🎉
// ✗

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	for idx := len(ts.testers) - 1; idx >= 0; idx-- {
		tt := ts.testers[idx]
		if !tt.Enabled() {
			continue
		}
		if err := tt.Delete(); err != nil {
			errs = append(errs, err.Error())
		}
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
