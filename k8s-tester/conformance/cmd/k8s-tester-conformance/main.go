// k8s-tester-conformance installs Kubernetes conformance tester.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-k8s-tester/k8s-tester/conformance"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-conformance",
	Short:      "Kubernetes conformance tester",
	SuggestFor: []string{"conformance"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt             bool
	logLevel           string
	logOutputs         []string
	minimumNodes       int
	namespace          string
	kubectlDownloadURL string
	kubectlPath        string
	kubeconfigPath     string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", conformance.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-conformance failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	sonobuoyPath                    string
	sonobuoyDownloadURL             string
	sonobuoyRunTimeout              time.Duration
	sonobuoyDeleteTimeout           time.Duration
	sonobuoyRunMode                 string
	sonobuoyRunE2EFocus             string
	sonobuoyRunE2ESkip              string
	sonobuoyRunKubeConformanceImage string
	sonobuoyRunE2ERepoConfig        string
	sonobuoyRunImage                string
	sonobuoyRunSystemdLogsImage     string
	sonobuoyResultsTarGzPath        string
	sonobuoyResultsE2ELogPath       string
	sonobuoyResultsJunitXMLPath     string
	sonobuoyResultsOutputDir        string
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	rootCmd.PersistentFlags().StringVar(&sonobuoyPath, "sonobuoy-path", conformance.DefaultSonobuoyPath(), "sonobuoy path")
	rootCmd.PersistentFlags().StringVar(&sonobuoyDownloadURL, "sonobuoy-download-url", conformance.DefaultSonobuoyDownloadURL(), "sonobuoy download URL")
	rootCmd.PersistentFlags().DurationVar(&sonobuoyRunTimeout, "sonobuoy-run-timeout", conformance.DefaultSonobuoyRunTimeout, "sonobuoy run timeout")
	rootCmd.PersistentFlags().DurationVar(&sonobuoyDeleteTimeout, "sonobuoy-delete timeout", conformance.DefaultSonobuoyDeleteTimeout, "sonobuoy delete timeout")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunMode, "sonobuoy-run-mode", conformance.DefaultSonobuoyRunMode, "sonobuoy run mode")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunE2EFocus, "sonobuoy-run-e2e-focus", "", "sonobuoy run e2e focus")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunE2ESkip, "sonobuoy-run-e2e-skip", "", "sonobuoy run e2e skip")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunKubeConformanceImage, "sonobuoy-run-kube-conformance-image", conformance.DefaultSonobuoyRunKubeConformanceImage, "sonobuoy run kube conformance image")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunE2ERepoConfig, "sonobuoy-run-e2e-repo-config", "", "sonobuoy run e2e repo config")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunImage, "sonobuoy-run-image", "", "sonobuoy run image")
	rootCmd.PersistentFlags().StringVar(&sonobuoyRunSystemdLogsImage, "sonobuoy-run-systemd-logs-image", "", "sonobuoy run systemd logs image")
	rootCmd.PersistentFlags().StringVar(&sonobuoyResultsTarGzPath, "sonobuoy-results-tar-gz-path", "", "sonobuoy results tar.gz path")
	rootCmd.PersistentFlags().StringVar(&sonobuoyResultsE2ELogPath, "sonobuoy-results-e2e-log-path", "", "sonobuoy e2e log path")
	rootCmd.PersistentFlags().StringVar(&sonobuoyResultsJunitXMLPath, "sonobuoy-results-junit-xml-path", "", "sonobuoy results Junit XML path")
	rootCmd.PersistentFlags().StringVar(&sonobuoyResultsOutputDir, "sonobuoy-results-output-dir", "", "sonobuoy results output dir")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &conformance.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Namespace:    namespace,
		Client:       cli,

		SonobuoyPath:                    sonobuoyPath,
		SonobuoyDownloadURL:             sonobuoyDownloadURL,
		SonobuoyRunTimeout:              sonobuoyRunTimeout,
		SonobuoyDeleteTimeout:           sonobuoyDeleteTimeout,
		SonobuoyRunMode:                 sonobuoyRunMode,
		SonobuoyRunE2EFocus:             sonobuoyRunE2EFocus,
		SonobuoyRunE2ESkip:              sonobuoyRunE2ESkip,
		SonobuoyRunKubeConformanceImage: sonobuoyRunKubeConformanceImage,
		SonobuoyRunE2ERepoConfig:        sonobuoyRunE2ERepoConfig,
		SonobuoyRunImage:                sonobuoyRunImage,
		SonobuoyRunSystemdLogsImage:     sonobuoyRunSystemdLogsImage,
		SonobuoyResultsTarGzPath:        sonobuoyResultsTarGzPath,
		SonobuoyResultsE2ELogPath:       sonobuoyResultsE2ELogPath,
		SonobuoyResultsJunitXMLPath:     sonobuoyResultsJunitXMLPath,
		SonobuoyResultsOutputDir:        sonobuoyResultsOutputDir,
	}

	ts := conformance.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-conformance apply' success\n")
}

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources",
		Run:   createDeleteFunc,
	}
	return cmd
}

func createDeleteFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &conformance.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Namespace: namespace,
		Client:    cli,

		SonobuoyPath:          sonobuoyPath,
		SonobuoyDownloadURL:   sonobuoyDownloadURL,
		SonobuoyDeleteTimeout: sonobuoyDeleteTimeout,
	}

	ts := conformance.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-conformance delete' success\n")
}
