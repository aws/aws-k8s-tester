package kubeadm

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/internal/kubeadm"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
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
		Short: "Delete kubeadm cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := kubeadmconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var tester kubeadm.Tester
	tester, err = kubeadm.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create kubeadm cluster %v\n", err)
		os.Exit(1)
	}

	if err = tester.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate cluster %v", err)
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester kubeadm delete cluster' success")
}
