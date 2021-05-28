// k8s-tester-jobs-echo installs Kubernetes Jobs echo tester.
package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/client"
	jobs_echo "github.com/aws/aws-k8s-tester/k8s-tester/jobs-echo"
	aws_v1 "github.com/aws/aws-k8s-tester/utils/aws/v1"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:        "k8s-tester-jobs-echo",
	Short:      "Kubernetes Jobs echo tester",
	SuggestFor: []string{"jobs-echo"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

var (
	enablePrompt   bool
	logLevel       string
	logOutputs     []string
	minimumNodes   int
	namespace      string
	kubectlPath    string
	kubeConfigPath string
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&enablePrompt, "enable-prompt", true, "'true' to enable prompt mode")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", log.DefaultLogLevel, "Logging level")
	rootCmd.PersistentFlags().StringSliceVar(&logOutputs, "log-outputs", []string{"stderr"}, "Additional logger outputs")
	rootCmd.PersistentFlags().IntVar(&minimumNodes, "minimum-nodes", jobs_echo.DefaultMinimumNodes, "minimum number of Kubernetes nodes required for installing this addon")
	rootCmd.PersistentFlags().StringVar(&namespace, "namespace", "test-namespace", "'true' to auto-generate path for create config/cluster, overwrites existing --path value")
	rootCmd.PersistentFlags().StringVar(&kubectlPath, "kubectl-path", "", "kubectl path")
	rootCmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig-path", "", "KUBECONFIG path")

	rootCmd.AddCommand(
		newApply(),
		newDelete(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-tester-jobs-echo failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

var (
	repositoryBusyboxPartition string
	repositoryBusyboxAccountID string
	repositoryBusyboxRegion    string
	repositoryBusyboxName      string
	repositoryBusyboxImageTag  string

	completes int32
	parallels int32
	echoSize  int32
)

func newApply() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply tests",
		Run:   createApplyFunc,
	}

	cmd.PersistentFlags().StringVar(&repositoryBusyboxPartition, "repository-busybox-partition", "", `used for deciding between "amazonaws.com" and "amazonaws.com.cn"`)
	cmd.PersistentFlags().StringVar(&repositoryBusyboxAccountID, "repository-busybox-account-id", "", "account ID for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryBusyboxRegion, "repository-busybox-region", "", "ECR repository region to pull from")
	cmd.PersistentFlags().StringVar(&repositoryBusyboxName, "repository-busybox-name", "", "repository name for tester ECR image")
	cmd.PersistentFlags().StringVar(&repositoryBusyboxImageTag, "repository-busybox-image-tag", "", "image tag for tester ECR image")

	cmd.PersistentFlags().Int32Var(&completes, "completes", jobs_echo.DefaultCompletes, "desired number of successfully finished pods")
	cmd.PersistentFlags().Int32Var(&parallels, "parallels", jobs_echo.DefaultParallels, "maximum desired number of pods the job should run at any given time")
	cmd.PersistentFlags().Int32Var(&echoSize, "echo-size", jobs_echo.DefaultEchoSize, "maximum desired number of pods the job should run at any given time")

	return cmd
}

func createApplyFunc(cmd *cobra.Command, args []string) {
	lg, logWriter, _, err := log.NewWithStderrWriter(logLevel, logOutputs)
	if err != nil {
		panic(err)
	}
	_ = zap.ReplaceGlobals(lg)

	cfg := jobs_echo.Config{
		EnablePrompt: enablePrompt,
		Logger:       lg,
		LogWriter:    logWriter,
		MinimumNodes: minimumNodes,
		Namespace:    namespace,
		ClientConfig: &client.Config{
			Logger:         lg,
			KubectlPath:    kubectlPath,
			KubeConfigPath: kubeConfigPath,
		},

		RepositoryBusyboxPartition: repositoryBusyboxPartition,
		RepositoryBusyboxAccountID: repositoryBusyboxAccountID,
		RepositoryBusyboxRegion:    repositoryBusyboxRegion,
		RepositoryBusyboxName:      repositoryBusyboxName,
		RepositoryBusyboxImageTag:  repositoryBusyboxImageTag,

		Completes: completes,
		Parallels: parallels,
		EchoSize:  echoSize,
	}

	if repositoryBusyboxPartition != "" &&
		repositoryBusyboxAccountID != "" &&
		repositoryBusyboxRegion != "" &&
		repositoryBusyboxName != "" &&
		repositoryBusyboxImageTag != "" {
		awsCfg := aws_v1.Config{
			Logger:        lg,
			DebugAPICalls: logLevel == "debug",
			Partition:     repositoryBusyboxPartition,
			Region:        repositoryBusyboxRegion,
		}
		awsSession, _, _, err := aws_v1.New(&awsCfg)
		if err != nil {
			panic(err)
		}
		cfg.ECRAPI = ecr.New(awsSession, aws.NewConfig().WithRegion(repositoryBusyboxRegion))
	}

	ts := jobs_echo.New(cfg)
	if err := ts.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-jobs-echo apply' success\n")
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

	cfg := jobs_echo.Config{
		EnablePrompt: enablePrompt,
		Logger:       lg,
		LogWriter:    logWriter,
		Namespace:    namespace,
		ClientConfig: &client.Config{
			Logger:         lg,
			KubectlPath:    kubectlPath,
			KubeConfigPath: kubeConfigPath,
		},
	}

	ts := jobs_echo.New(cfg)
	if err := ts.Delete(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to delete (%v)\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'k8s-tester-jobs-echo delete' success\n")
}
