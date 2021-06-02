// k8s-tester-php-apache installs Kubernetes NLB hello world tester.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	php_apache "github.com/aws/aws-k8s-tester/k8s-tester/php-apache"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-php-apache",
	Short:      "Kubernetes PHP Apache tester",
	SuggestFor: []string{"php-apache"},
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
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", php_apache.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
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
		fmt.Fprintf(os.Stderr, "k8s-tester-php-apache failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	repositoryPartition string
	repositoryAccountID string
	repositoryRegion    string
	repositoryName      string
	repositoryImageTag  string

	deploymentNodeSelector string
	deploymentReplicas     int32
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	cmd.PersistentFlags().StringVar(&repositoryPartition, "repository-partition", "", `used for deciding between "amazonaws.com" and "amazonaws.com.cn"`)
	cmd.PersistentFlags().StringVar(&repositoryAccountID, "repository-account-id", "", "account ID for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryRegion, "repository-region", "", "ECR repository region to pull from")
	cmd.PersistentFlags().StringVar(&repositoryName, "repository-name", "", "repository name for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryImageTag, "repository-image-tag", "", "image tag for tester ECR image")
	cmd.PersistentFlags().StringVar(&deploymentNodeSelector, "deployment-node-selector", "", "map of deployment node selector, must be valid JSON format")
	cmd.PersistentFlags().Int32Var(&deploymentReplicas, "deployment-replicas", php_apache.DefaultDeploymentReplicas, "number of deployment replicas")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	nodeSelector := make(map[string]string)
	if err := json.Unmarshal([]byte(deploymentNodeSelector), &nodeSelector); err != nil {
		lg.Panic("failed to parse", zap.String("deloyment-node-selector", deploymentNodeSelector), zap.Error(err))
	}

	cli, err := client.New(&client.Config{
		Logger:             lg,
		KubectlDownloadURL: kubectlDownloadURL,
		KubectlPath:        kubectlPath,
		KubeconfigPath:     kubeconfigPath,
	})
	if err != nil {
		lg.Panic("failed to create client", zap.Error(err))
	}

	cfg := &php_apache.Config{
		Prompt:       prompt,
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Namespace:    namespace,
		Client:       cli,

		RepositoryPartition: repositoryPartition,
		RepositoryAccountID: repositoryAccountID,
		RepositoryRegion:    repositoryRegion,
		RepositoryName:      repositoryName,
		RepositoryImageTag:  repositoryImageTag,

		DeploymentNodeSelector: nodeSelector,
		DeploymentReplicas:     deploymentReplicas,
	}

	ts := php_apache.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-php-apache apply' success\n")
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

	cfg := &php_apache.Config{
		Prompt:    prompt,
		Logger:    lg,
		LogWriter: logWriter,
		Namespace: namespace,
		Client:    cli,
	}

	ts := php_apache.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-php-apache delete' success\n")
}
