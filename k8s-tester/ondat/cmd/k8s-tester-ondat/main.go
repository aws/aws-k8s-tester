// k8s-tester-ondat installs ondat agent using helm, and tests that it's able to function correctly.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	ondat "github.com/aws/aws-k8s-tester/k8s-tester/ondat"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-ondat",
	Short:      "Kubernetes ondat tester",
	SuggestFor: []string{"ondat"},
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
	etcdNamespace      string
	ondatNamespace     string
	etcdReplicas       int
	etcdStorage        int
	etcdStorageClass   string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", ondat.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlDownloadURL, "kubectl-download-url", client.DefaultKubectlDownloadURL(), "kubectl download URL")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", client.DefaultKubectlPath(), "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig-path", "", "KUBECONFIG path")
	rootCmd.PersistentFlags().StringVar(&etcdNamespace, "etcd-namespace", ondat.DefaultEtcdNamespace, "namespace for etcd, default storageos-etcd")
	rootCmd.PersistentFlags().StringVar(&ondatNamespace, "ondat-namespace", ondat.DefaultOndatNamespace, "namespace for ondat, default storageos")
	rootCmd.PersistentFlags().IntVar(&etcdReplicas, "etcd-replicas", ondat.DefaultEtcdReplicas, "total number of replicas of etcd to run, should be equal to or smaller than minimum-nodes")
	rootCmd.PersistentFlags().StringVar(&etcdStorageClass, "etcd-storageclass", "", "storageclass for etcd, cannot be storageos. required.")
	rootCmd.PersistentFlags().IntVar(&etcdStorage, "etcd-storage", ondat.DefaultEtcdStorage, "amount of GiB to assign each etcd replica, default 12")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-ondat failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var helmChartRepoURL string

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}
	cmd.PersistentFlags().StringVar(&helmChartRepoURL, "helm-chart-repo-url", ondat.DefaultHelmChartRepoURL, "helm chart repo URL")
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

	cfg := &ondat.Config{
		Prompt:           prompt,
		Logger:           lg,
		LogWriter:        logWriter,
		MinimumNodes:     minimumNodes,
		Namespace:        namespace,
		HelmChartRepoURL: helmChartRepoURL,
		Client:           cli,
		OndatNamespace:   ondatNamespace,
		EtcdNamespace:    etcdNamespace,
		EtcdReplicas:     etcdReplicas,
		EtcdStorage:      etcdStorage,
		EtcdStorageClass: etcdStorageClass,
	}

	ts := ondat.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-ondat apply' success\n")
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

	cfg := &ondat.Config{
		Prompt:           prompt,
		Logger:           lg,
		LogWriter:        logWriter,
		Namespace:        namespace,
		Client:           cli,
		OndatNamespace:   ondatNamespace,
		EtcdNamespace:    etcdNamespace,
		EtcdReplicas:     etcdReplicas,
		EtcdStorage:      etcdStorage,
		EtcdStorageClass: etcdStorageClass,
	}

	ts := ondat.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-ondat delete' success\n")
}
