// k8s-tester-stress installs Kubernetes stress tester.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-k8s-tester/k8s-tester/stress"
	aws_v1_ecr "github.com/aws/aws-k8s-tester/utils/aws/v1/ecr"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-stress",
	Short:      "Kubernetes stress tester",
	SuggestFor: []string{"stress"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	prompt                bool
	logLevel              string
	logOutputs            []string
	minimumNodes          int
	namespace             string
	skipNamespaceCreation bool
	kubectlDownloadURL    string
	kubectlPath           string
	kubeconfigPath        string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&prompt, "prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", stress.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().BoolVar(&skipNamespaceCreation, "skip-namespace-creation", stress.DefaultSkipNamespaceCreation, "'true' to skip namespace creation")
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
		fmt.Fprintf(os.Stderr, "k8s-tester-stress failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	ecrBusyBoxImage     string
	repositoryPartition string
	repositoryAccountID string
	repositoryRegion    string
	repositoryName      string
	repositoryImageTag  string

	runTimeout        time.Duration
	clients           int
	objectKeyPrefix   string
	objects           int
	objectSize        int
	updateConcurrency int
	listBatchLimit    int64
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	cmd.PersistentFlags().StringVar(&ecrBusyBoxImage, "ecr-busybox-image", "", "if not empty, we skip ECR image describe")
	cmd.PersistentFlags().StringVar(&repositoryPartition, "repository-partition", "", `used for deciding between "amazonaws.com" and "amazonaws.com.cn"`)
	cmd.PersistentFlags().StringVar(&repositoryAccountID, "repository-account-id", "", "account ID for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryRegion, "repository-region", "", "ECR repository region to pull from")
	cmd.PersistentFlags().StringVar(&repositoryName, "repository-name", "", "repository name for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryImageTag, "repository-image-tag", "", "image tag for tester ECR image")

	cmd.PersistentFlags().DurationVar(&runTimeout, "run-timeout", stress.DefaultRunTimeout, "run timeout")
	cmd.PersistentFlags().IntVar(&clients, "clients", 5, "number of clients")
	cmd.PersistentFlags().StringVar(&objectKeyPrefix, "object-key-prefix", stress.DefaultObjectKeyPrefix(), "object key prefix")
	cmd.PersistentFlags().IntVar(&objects, "objects", stress.DefaultObjects, "number of objects")
	cmd.PersistentFlags().IntVar(&objectSize, "object-size", stress.DefaultObjectSize, "object size")
	cmd.PersistentFlags().IntVar(&updateConcurrency, "update-concurrency", stress.DefaultUpdateConcurrency, "update concurrency")
	cmd.PersistentFlags().Int64Var(&listBatchLimit, "list-batch-limit", stress.DefaultListBatchLimit, "list limit")

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
		Clients:            clients,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &stress.Config{
		Prompt:                prompt,
		Logger:                lg,
		LogWriter:             logWriter,
		MinimumNodes:          minimumNodes,
		Namespace:             namespace,
		SkipNamespaceCreation: skipNamespaceCreation,

		ECRBusyboxImage: ecrBusyBoxImage,
		Repository: &aws_v1_ecr.Repository{
			Partition: repositoryPartition,
			AccountID: repositoryAccountID,
			Region:    repositoryRegion,
			Name:      repositoryName,
			ImageTag:  repositoryImageTag,
		},

		Client:            cli,
		RunTimeout:        runTimeout,
		ObjectKeyPrefix:   objectKeyPrefix,
		Objects:           objects,
		ObjectSize:        objectSize,
		UpdateConcurrency: updateConcurrency,
		ListBatchLimit:    listBatchLimit,
	}

	ts := stress.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-stress apply' success\n")
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

	cfg := &stress.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Namespace: namespace,
		Client:    cli,
	}

	ts := stress.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-stress delete' success\n")
}
