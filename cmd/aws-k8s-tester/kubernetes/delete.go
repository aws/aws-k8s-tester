package kubernetes

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/internal/kubernetes"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/spf13/cobra"
)

func newDelete() *cobra.Command {
	ac := &cobra.Command{
		Use:   "delete <subcommand>",
		Short: "Delete commands",
	}
	ac.AddCommand(newDeleteCluster())
	return ac
}

func newDeleteCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Delete Kubernetes cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := kubernetesconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var dp kubernetes.Deployer
	dp, err = kubernetes.NewDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create kubernetes cluster %v\n", err)
		os.Exit(1)
	}

	if err = dp.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate cluster %v", err)
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester kubernetes delete cluster' success")
}
