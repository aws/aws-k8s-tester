// Package k8s_tester implements k8s-tester.
// Same run order as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617.
package k8s_tester

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
	"github.com/aws/aws-k8s-tester/k8s-tester/clusterloader"
	cni "github.com/aws/aws-k8s-tester/k8s-tester/cni"
	"github.com/aws/aws-k8s-tester/k8s-tester/configmaps"
	"github.com/aws/aws-k8s-tester/k8s-tester/conformance"
	csi_ebs "github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs"
	"github.com/aws/aws-k8s-tester/k8s-tester/csrs"
	falco "github.com/aws/aws-k8s-tester/k8s-tester/falco"
	"github.com/aws/aws-k8s-tester/k8s-tester/falcon"
	securecn "github.com/aws/aws-k8s-tester/k8s-tester/secureCN"
	"github.com/aws/aws-k8s-tester/k8s-tester/falcon"
	fluent_bit "github.com/aws/aws-k8s-tester/k8s-tester/fluent-bit"
	jobs_echo "github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo"
	jobs_pi "github.com/aws/aws-k8s-tester/k8s-tester/jobs-pi"
	kubernetes_dashboard "github.com/aws/aws-k8s-tester/k8s-tester/kubernetes-dashboard"
	metrics_server "github.com/aws/aws-k8s-tester/k8s-tester/metrics-server"
	nlb_guestbook "github.com/aws/aws-k8s-tester/k8s-tester/nlb-guestbook"
	nlb_hello_world "github.com/aws/aws-k8s-tester/k8s-tester/nlb-hello-world"
	php_apache "github.com/aws/aws-k8s-tester/k8s-tester/php-apache"
	"github.com/aws/aws-k8s-tester/k8s-tester/secrets"
	"github.com/aws/aws-k8s-tester/k8s-tester/stress"
	stress_in_cluster "github.com/aws/aws-k8s-tester/k8s-tester/stress/in-cluster"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/k8s-tester/version"
	"github.com/aws/aws-k8s-tester/k8s-tester/wordpress"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
)

func New(cfg *Config) k8s_tester.Tester {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		panic(fmt.Errorf("failed to validate config %v", err))
	}

	lg, logWriter, logFile, err := log.NewWithStderrWriter(cfg.LogLevel, cfg.LogOutputs)
	if err != nil {
		panic(fmt.Errorf("failed to create logger %v", err))
	}
	_ = zap.ReplaceGlobals(lg)

	ts := &tester{
		color: cfg.Colorize,

		stopCreationCh:     make(chan struct{}),
		stopCreationChOnce: new(sync.Once),
		osSig:              make(chan os.Signal),
		deleteMu:           new(sync.Mutex),

		cfg:       cfg,
		logger:    lg,
		logWriter: logWriter,
		logFile:   logFile,
		testers:   make([]k8s_tester.Tester, 0),
	}
	signal.Notify(ts.osSig, syscall.SIGTERM, syscall.SIGINT)

	fmt.Fprint(logWriter, ts.color("\n\n\n[yellow]*********************************\n"))
	fmt.Fprintln(logWriter, "😎 🙏 🚶 ✔️ 👍")
	fmt.Fprintf(logWriter, ts.color("[light_green]New k8s-tester %q [default](%q)\n\n"), cfg.ConfigPath, version.Version())

	ts.cli, err = client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: cfg.KubectlDownloadURL,
		KubectlPath:        cfg.KubectlPath,
		KubeconfigPath:     cfg.KubeconfigPath,
		KubeconfigContext:  cfg.KubeconfigContext,
		Clients:            cfg.Clients,
		ClientQPS:          cfg.ClientQPS,
		ClientBurst:        cfg.ClientBurst,
		ClientTimeout:      cfg.ClientTimeout,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	ts.createTesters()

	return ts
}

type tester struct {
	color func(string) string

	stopCreationCh     chan struct{}
	stopCreationChOnce *sync.Once
	osSig              chan os.Signal
	deleteMu           *sync.Mutex
	logger             *zap.Logger
	logWriter          io.Writer
	logFile            *os.File
	cli                client.Client

	cfg *Config

	// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
	testers []k8s_tester.Tester
}

func (ts *tester) createTesters() {
	fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
	fmt.Fprintf(ts.logWriter, ts.color("[light_green]createTesters [default](%q)\n"), ts.cfg.ConfigPath)

	// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
	if ts.cfg.AddOnCloudwatchAgent != nil && ts.cfg.AddOnCloudwatchAgent.Enable {
		ts.cfg.AddOnCloudwatchAgent.Stopc = ts.stopCreationCh
		ts.cfg.AddOnCloudwatchAgent.Logger = ts.logger
		ts.cfg.AddOnCloudwatchAgent.LogWriter = ts.logWriter
		ts.cfg.AddOnCloudwatchAgent.Client = ts.cli
		ts.testers = append(ts.testers, cloudwatch_agent.New(ts.cfg.AddOnCloudwatchAgent))
	}
	if ts.cfg.AddOnFluentBit != nil && ts.cfg.AddOnFluentBit.Enable {
		ts.cfg.AddOnFluentBit.Stopc = ts.stopCreationCh
		ts.cfg.AddOnFluentBit.Logger = ts.logger
		ts.cfg.AddOnFluentBit.LogWriter = ts.logWriter
		ts.cfg.AddOnFluentBit.Client = ts.cli
		ts.testers = append(ts.testers, fluent_bit.New(ts.cfg.AddOnFluentBit))
	}
	if ts.cfg.AddOnMetricsServer != nil && ts.cfg.AddOnMetricsServer.Enable {
		ts.cfg.AddOnMetricsServer.Stopc = ts.stopCreationCh
		ts.cfg.AddOnMetricsServer.Logger = ts.logger
		ts.cfg.AddOnMetricsServer.LogWriter = ts.logWriter
		ts.cfg.AddOnMetricsServer.Client = ts.cli
		ts.testers = append(ts.testers, metrics_server.New(ts.cfg.AddOnMetricsServer))
	}
	if ts.cfg.AddOnCNI != nil && ts.cfg.AddOnCNI.Enable {
		ts.cfg.AddOnCNI.Stopc = ts.stopCreationCh
		ts.cfg.AddOnCNI.Logger = ts.logger
		ts.cfg.AddOnCNI.LogWriter = ts.logWriter
		ts.cfg.AddOnCNI.Client = ts.cli
		ts.testers = append(ts.testers, cni.New(ts.cfg.AddOnCNI))
	}
	if ts.cfg.AddOnConformance != nil && ts.cfg.AddOnConformance.Enable {
		ts.cfg.AddOnConformance.Stopc = ts.stopCreationCh
		ts.cfg.AddOnConformance.Logger = ts.logger
		ts.cfg.AddOnConformance.LogWriter = ts.logWriter
		ts.cfg.AddOnConformance.Client = ts.cli
		ts.testers = append(ts.testers, conformance.New(ts.cfg.AddOnConformance))
	}
	if ts.cfg.AddOnCSIEBS != nil && ts.cfg.AddOnCSIEBS.Enable {
		ts.cfg.AddOnCSIEBS.Stopc = ts.stopCreationCh
		ts.cfg.AddOnCSIEBS.Logger = ts.logger
		ts.cfg.AddOnCSIEBS.LogWriter = ts.logWriter
		ts.cfg.AddOnCSIEBS.Client = ts.cli
		ts.testers = append(ts.testers, csi_ebs.New(ts.cfg.AddOnCSIEBS))
	}
	if ts.cfg.AddOnKubernetesDashboard != nil && ts.cfg.AddOnKubernetesDashboard.Enable {
		ts.cfg.AddOnKubernetesDashboard.Stopc = ts.stopCreationCh
		ts.cfg.AddOnKubernetesDashboard.Logger = ts.logger
		ts.cfg.AddOnKubernetesDashboard.LogWriter = ts.logWriter
		ts.cfg.AddOnKubernetesDashboard.Client = ts.cli
		ts.testers = append(ts.testers, kubernetes_dashboard.New(ts.cfg.AddOnKubernetesDashboard))
	}
	if ts.cfg.AddOnPHPApache != nil && ts.cfg.AddOnPHPApache.Enable {
		ts.cfg.AddOnPHPApache.Stopc = ts.stopCreationCh
		ts.cfg.AddOnPHPApache.Logger = ts.logger
		ts.cfg.AddOnPHPApache.LogWriter = ts.logWriter
		ts.cfg.AddOnPHPApache.Client = ts.cli
		ts.testers = append(ts.testers, php_apache.New(ts.cfg.AddOnPHPApache))
	}
	if ts.cfg.AddOnNLBGuestbook != nil && ts.cfg.AddOnNLBGuestbook.Enable {
		ts.cfg.AddOnNLBGuestbook.Stopc = ts.stopCreationCh
		ts.cfg.AddOnNLBGuestbook.Logger = ts.logger
		ts.cfg.AddOnNLBGuestbook.LogWriter = ts.logWriter
		ts.cfg.AddOnNLBGuestbook.Client = ts.cli
		ts.testers = append(ts.testers, nlb_guestbook.New(ts.cfg.AddOnNLBGuestbook))
	}
	if ts.cfg.AddOnNLBHelloWorld != nil && ts.cfg.AddOnNLBHelloWorld.Enable {
		ts.cfg.AddOnNLBHelloWorld.Stopc = ts.stopCreationCh
		ts.cfg.AddOnNLBHelloWorld.Logger = ts.logger
		ts.cfg.AddOnNLBHelloWorld.LogWriter = ts.logWriter
		ts.cfg.AddOnNLBHelloWorld.Client = ts.cli
		ts.testers = append(ts.testers, nlb_hello_world.New(ts.cfg.AddOnNLBHelloWorld))
	}
	if ts.cfg.AddOnWordpress != nil && ts.cfg.AddOnWordpress.Enable {
		ts.cfg.AddOnWordpress.Stopc = ts.stopCreationCh
		ts.cfg.AddOnWordpress.Logger = ts.logger
		ts.cfg.AddOnWordpress.LogWriter = ts.logWriter
		ts.cfg.AddOnWordpress.Client = ts.cli
		ts.testers = append(ts.testers, wordpress.New(ts.cfg.AddOnWordpress))
	}
	if ts.cfg.AddOnJobsPi != nil && ts.cfg.AddOnJobsPi.Enable {
		ts.cfg.AddOnJobsPi.Stopc = ts.stopCreationCh
		ts.cfg.AddOnJobsPi.Logger = ts.logger
		ts.cfg.AddOnJobsPi.LogWriter = ts.logWriter
		ts.cfg.AddOnJobsPi.Client = ts.cli
		ts.testers = append(ts.testers, jobs_pi.New(ts.cfg.AddOnJobsPi))
	}
	if ts.cfg.AddOnJobsEcho != nil && ts.cfg.AddOnJobsEcho.Enable {
		ts.cfg.AddOnJobsEcho.Stopc = ts.stopCreationCh
		ts.cfg.AddOnJobsEcho.Logger = ts.logger
		ts.cfg.AddOnJobsEcho.LogWriter = ts.logWriter
		ts.cfg.AddOnJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(ts.cfg.AddOnJobsEcho))
	}
	if ts.cfg.AddOnCronJobsEcho != nil && ts.cfg.AddOnCronJobsEcho.Enable {
		ts.cfg.AddOnCronJobsEcho.Stopc = ts.stopCreationCh
		ts.cfg.AddOnCronJobsEcho.Logger = ts.logger
		ts.cfg.AddOnCronJobsEcho.LogWriter = ts.logWriter
		ts.cfg.AddOnCronJobsEcho.Client = ts.cli
		ts.testers = append(ts.testers, jobs_echo.New(ts.cfg.AddOnCronJobsEcho))
	}
	if ts.cfg.AddOnCSRs != nil && ts.cfg.AddOnCSRs.Enable {
		ts.cfg.AddOnCSRs.Stopc = ts.stopCreationCh
		ts.cfg.AddOnCSRs.Logger = ts.logger
		ts.cfg.AddOnCSRs.LogWriter = ts.logWriter
		ts.cfg.AddOnCSRs.Client = ts.cli
		ts.testers = append(ts.testers, csrs.New(ts.cfg.AddOnCSRs))
	}
	if ts.cfg.AddOnConfigmaps != nil && ts.cfg.AddOnConfigmaps.Enable {
		ts.cfg.AddOnConfigmaps.Stopc = ts.stopCreationCh
		ts.cfg.AddOnConfigmaps.Logger = ts.logger
		ts.cfg.AddOnConfigmaps.LogWriter = ts.logWriter
		ts.cfg.AddOnConfigmaps.Client = ts.cli
		ts.testers = append(ts.testers, configmaps.New(ts.cfg.AddOnConfigmaps))
	}
	if ts.cfg.AddOnSecrets != nil && ts.cfg.AddOnSecrets.Enable {
		ts.cfg.AddOnSecrets.Stopc = ts.stopCreationCh
		ts.cfg.AddOnSecrets.Logger = ts.logger
		ts.cfg.AddOnSecrets.LogWriter = ts.logWriter
		ts.cfg.AddOnSecrets.Client = ts.cli
		ts.testers = append(ts.testers, secrets.New(ts.cfg.AddOnSecrets))
	}
	if ts.cfg.AddOnClusterloader != nil && ts.cfg.AddOnClusterloader.Enable {
		ts.cfg.AddOnClusterloader.Stopc = ts.stopCreationCh
		ts.cfg.AddOnClusterloader.Logger = ts.logger
		ts.cfg.AddOnClusterloader.LogWriter = ts.logWriter
		ts.cfg.AddOnClusterloader.Client = ts.cli
		ts.testers = append(ts.testers, clusterloader.New(ts.cfg.AddOnClusterloader))
	}
	if ts.cfg.AddOnStress != nil && ts.cfg.AddOnStress.Enable {
		ts.cfg.AddOnStress.Stopc = ts.stopCreationCh
		ts.cfg.AddOnStress.Logger = ts.logger
		ts.cfg.AddOnStress.LogWriter = ts.logWriter
		ts.cfg.AddOnStress.Client = ts.cli
		ts.testers = append(ts.testers, stress.New(ts.cfg.AddOnStress))
	}
	if ts.cfg.AddOnStressInCluster != nil && ts.cfg.AddOnStressInCluster.Enable {
		ts.cfg.AddOnStressInCluster.Stopc = ts.stopCreationCh
		ts.cfg.AddOnStressInCluster.Logger = ts.logger
		ts.cfg.AddOnStressInCluster.LogWriter = ts.logWriter
		ts.cfg.AddOnStressInCluster.Client = ts.cli
		ts.testers = append(ts.testers, stress_in_cluster.New(ts.cfg.AddOnStressInCluster))
	}
	if ts.cfg.AddOnFalco != nil && ts.cfg.AddOnFalco.Enable {
		ts.cfg.AddOnFalco.Stopc = ts.stopCreationCh
		ts.cfg.AddOnFalco.Logger = ts.logger
		ts.cfg.AddOnFalco.LogWriter = ts.logWriter
		ts.cfg.AddOnFalco.Client = ts.cli
		ts.testers = append(ts.testers, falco.New(ts.cfg.AddOnFalco))
	}
	if ts.cfg.AddOnFalcon != nil && ts.cfg.AddOnFalcon.Enable {
		ts.cfg.AddOnFalcon.Stopc = ts.stopCreationCh
		ts.cfg.AddOnFalcon.Logger = ts.logger
		ts.cfg.AddOnFalcon.LogWriter = ts.logWriter
		ts.cfg.AddOnFalcon.Client = ts.cli
		ts.testers = append(ts.testers, falcon.New(ts.cfg.AddOnFalcon))
	}
	if ts.cfg.AddOnSecureCN != nil && ts.cfg.AddOnSecureCN.Enable {
		ts.cfg.AddOnSecureCN.Stopc = ts.stopCreationCh
		ts.cfg.AddOnSecureCN.Logger = ts.logger
		ts.cfg.AddOnSecureCN.LogWriter = ts.logWriter
		ts.cfg.AddOnSecureCN.Client = ts.cli
		ts.testers = append(ts.testers, securecn.New(ts.cfg.AddOnSecureCN))
	}
	if ts.cfg.AddOnFalcon != nil && ts.cfg.AddOnFalcon.Enable {
		ts.cfg.AddOnFalcon.Stopc = ts.stopCreationCh
		ts.cfg.AddOnFalcon.Logger = ts.logger
		ts.cfg.AddOnFalcon.LogWriter = ts.logWriter
		ts.cfg.AddOnFalcon.Client = ts.cli
		ts.testers = append(ts.testers, falcon.New(ts.cfg.AddOnFalcon))
	}
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return true }

func (ts *tester) Apply() (err error) {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	nodes, err := client.ListNodes(ts.cli.KubernetesClient())
	if len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}
	ts.cfg.TotalNodes = len(nodes)
	ts.cfg.Sync()

	now := time.Now()
	defer func() {
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]Apply.defer [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprintf(ts.logWriter, "\n\n# to uninstall add-ons\nk8s-tester delete --path %s\n\n", ts.cfg.ConfigPath)
		ts.cfg.Sync()
		ts.logFile.Sync()

		if err == nil {
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_green]kubectl [default](%q)\n"), ts.cfg.ConfigPath)
			fmt.Fprintln(ts.logWriter, ts.cfg.KubectlCommands())

			ts.logger.Info("Apply succeeded",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

			ts.logger.Sugar().Infof("Apply.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprint(ts.logWriter, ts.color("\n\n💯 😁 👍 :) [light_green]Apply SUCCESS\n\n\n"))

			fmt.Fprintf(ts.logWriter, "\n\n# to uninstall add-ons\nk8s-tester delete --path %s\n\n", ts.cfg.ConfigPath)
			ts.cfg.Sync()
			ts.logFile.Sync()
			return
		}

		fmt.Fprintf(ts.logWriter, ts.color("\n\n\n[light_magenta]Apply FAIL ERROR:\n\n[default]%v\n\n\n"), err)
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]Apply FAIL\n"))
		fmt.Fprintf(ts.logWriter, "\n\n# to uninstall add-ons\nk8s-tester delete --path %s\n\n", ts.cfg.ConfigPath)
		ts.logger.Warn("Apply failed; reverting resource creation",
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		derr := ts.delete()
		if derr != nil {
			ts.logger.Warn("failed to revert Apply", zap.Error(derr))
		} else {
			ts.logger.Warn("reverted Apply")
		}
		fmt.Fprintf(ts.logWriter, ts.color("\n\n\n[light_magenta]Apply FAIL ERROR:\n\n[default]%v\n\n\n"), err)
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprint(ts.logWriter, ts.color("\n\n🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]Apply FAIL\n\n\n"))
		fmt.Fprintf(ts.logWriter, "\n\n# to uninstall add-ons\nk8s-tester delete --path %s\n\n", ts.cfg.ConfigPath)

		ts.logger.Sugar().Infof("Apply.defer end (%s, %s)", ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		ts.logFile.Sync()
	}()

	// tester order is defined as https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/eks.go#L617
	for idx, cur := range ts.testers {
		if !cur.Enabled() {
			continue
		}
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_green]testers[%02d].Apply [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		err = catchInterrupt(
			ts.logger,
			ts.stopCreationCh,
			ts.stopCreationChOnce,
			ts.osSig,
			cur.Apply,
			cur.Name(),
		)
		ts.cfg.Sync()
		if err != nil {
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_magenta]✗ [default]k8s-tester[%02d].Apply [light_magenta]FAIL [default](%v)\n"), idx, err)
			return err
		}
	}

	fmt.Fprint(ts.logWriter, ts.color("\n\n\n[yellow]*********************************\n"))
	fmt.Fprint(ts.logWriter, ts.color("🎉 [default]k8s-tester eks create cluster [light_green]SUCCESS\n"))
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	return ts.delete()
}

func (ts *tester) delete() error {
	ts.deleteMu.Lock()
	defer ts.deleteMu.Unlock()

	var errs []string

	now := time.Now()

	for idx := len(ts.testers) - 1; idx >= 0; idx-- {
		cur := ts.testers[idx]
		if !cur.Enabled() {
			continue
		}
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]testers[%02d].Delete [cyan]%q [default](%q, %q)\n"), idx, cur.Name(), ts.cfg.ConfigPath, ts.cfg.KubectlCommand())
		if err := cur.Delete(); err != nil {
			fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
			fmt.Fprintf(ts.logWriter, ts.color("[light_magenta]✗ [default]k8s-tester[%02d].Delete [light_magenta]FAIL [default](%v)\n"), idx, err)
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]Delete [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprint(ts.logWriter, ts.color("\n\n💯 😁 👍 :) [light_blue]Delete SUCCESS\n\n\n"))

		ts.logger.Info("successfully finished Delete",
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)

	} else {
		fmt.Fprint(ts.logWriter, ts.color("\n\n[yellow]*********************************\n"))
		fmt.Fprintf(ts.logWriter, ts.color("[light_blue]Delete [default](%q)\n"), ts.cfg.ConfigPath)
		fmt.Fprint(ts.logWriter, ts.color("🔥 💀 👽 😱 😡 ⛈   (-_-) [light_magenta]Delete FAIL\n"))

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

func catchInterrupt(lg *zap.Logger, stopc chan struct{}, stopcCloseOnce *sync.Once, osSigCh chan os.Signal, run func() error, name string) (err error) {
	errc := make(chan error)
	go func() {
		errc <- run()
	}()

	select {
	case _, ok := <-stopc:
		rerr := <-errc
		lg.Info("interrupted; stopc received, errc received", zap.Error(rerr))
		err = fmt.Errorf("stopc returned, stopc open %v, run function returned %v (%q)", ok, rerr, name)

	case osSig := <-osSigCh:
		stopcCloseOnce.Do(func() { close(stopc) })
		rerr := <-errc
		lg.Info("OS signal received, errc received", zap.String("signal", osSig.String()), zap.Error(rerr))
		err = fmt.Errorf("received os signal %v, closed stopc, run function returned %v (%q)", osSig, rerr, name)

	case err = <-errc:
		if err != nil {
			err = fmt.Errorf("run function returned %v (%q)", err, name)
		}
	}
	return err
}
