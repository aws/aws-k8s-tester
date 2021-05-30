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
	"time"

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
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
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

	ts := &tester{
		cfg:       cfg,
		logger:    lg,
		logWriter: logWriter,
		testers:   make([]k8s_tester.Tester, 0),
	}
	ts.cli, err = client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: cfg.KubectlDownloadURL,
		KubectlPath:        cfg.KubectlPath,
		KubeconfigPath:     cfg.KubeconfigPath,
		KubeconfigContext:  cfg.KubeconfigContext,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	ts.createTesters()

	return ts
}

type tester struct {
	cfg *Config

	logger    *zap.Logger
	logWriter io.Writer
	cli       client.Client

	// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
	testers []k8s_tester.Tester
}

func (ts *tester) createTesters() {
	fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("[light_green]createTesters [default](%q)\n"), ts.cfg.ConfigPath)

	// The tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eksconfig/env.go.
	if ts.cfg.AddOnCloudwatchAgent != nil && ts.cfg.AddOnCloudwatchAgent.Enable {
		ts.cfg.AddOnCloudwatchAgent.Client = ts.cli
		ts.testers = append(ts.testers, cloudwatch_agent.New(ts.cfg.AddOnCloudwatchAgent))
	}
	if ts.cfg.AddOnMetricsServer != nil && ts.cfg.AddOnMetricsServer.Enable {
		ts.cfg.AddOnMetricsServer.Client = ts.cli
		ts.testers = append(ts.testers, metrics_server.New(ts.cfg.AddOnMetricsServer))
	}
	if ts.cfg.AddOnFluentBit != nil && ts.cfg.AddOnFluentBit.Enable {
		ts.cfg.AddOnFluentBit.Client = ts.cli
		ts.testers = append(ts.testers, fluent_bit.New(ts.cfg.AddOnFluentBit))
	}
	if ts.cfg.AddOnKubernetesDashboard != nil && ts.cfg.AddOnKubernetesDashboard.Enable {
		ts.cfg.AddOnKubernetesDashboard.Client = ts.cli
		ts.testers = append(ts.testers, kubernetes_dashboard.New(ts.cfg.AddOnKubernetesDashboard))
	}
	if ts.cfg.AddOnNLBHelloWorld != nil && ts.cfg.AddOnNLBHelloWorld.Enable {
		ts.cfg.AddOnNLBHelloWorld.Client = ts.cli
		ts.testers = append(ts.testers, nlb_hello_world.New(ts.cfg.AddOnNLBHelloWorld))
	}
	if ts.cfg.AddOnJobsPi != nil && ts.cfg.AddOnJobsPi.Enable {
		ts.cfg.AddOnJobsPi.Client = ts.cli
		ts.testers = append(ts.testers, jobs_pi.New(ts.cfg.AddOnJobsPi))
	}
	if ts.cfg.AddOnJobsEcho != nil && ts.cfg.AddOnJobsEcho.Enable {
		ts.cfg.AddOnJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(ts.cfg.AddOnJobsEcho))
	}
	if ts.cfg.AddOnCronJobsEcho != nil && ts.cfg.AddOnCronJobsEcho.Enable {
		ts.cfg.AddOnCronJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(ts.cfg.AddOnCronJobsEcho))
	}
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
	for idx, cur := range ts.testers {
		_ = idx
		if !cur.Enabled() {
			continue
		}
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("[light_green]testers[%02d].Apply [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := cur.Apply(); err != nil {
			fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.cfg.Colorize(fmt.Sprintf("[light_magenta]✗ [default]k8s-tester[%02d].Apply [light_magenta]FAIL [default](%v)\n", idx, err)))
			return err
		}
	}

	fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("🎉 [default]k8s-tester eks create cluster [light_green]SUCCESS\n"))
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	now := time.Now()

	for idx := len(ts.testers) - 1; idx >= 0; idx-- {
		cur := ts.testers[idx]
		if !cur.Enabled() {
			continue
		}
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("[light_blue]testers[%02d].Delete [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := cur.Delete(); err != nil {
			fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.cfg.Colorize(fmt.Sprintf("[light_magenta]✗ [default]k8s-tester[%02d].Delete [light_magenta]FAIL [default](%v)\n", idx, err)))
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("[light_blue]Delete [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n💯 😁 👍 :) [light_blue]Delete SUCCESS\n\n\n"))

		ts.logger.Info("successfully finished Delete",
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)

	} else {
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("[light_blue]Delete [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprintf(ts.logWriter, ts.cfg.Colorize("🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]Delete FAIL\n"))

		ts.logger.Info("failed Delete",
			zap.Strings("errors", errs),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
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
