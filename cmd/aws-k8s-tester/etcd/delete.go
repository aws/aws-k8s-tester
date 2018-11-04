package etcd

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/etcdtester"
	"github.com/aws/aws-k8s-tester/internal/etcd"
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
		Short: "Delete etcd cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := etcdconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var tester etcdtester.Tester
	tester, err = etcd.NewTester(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create etcd cluster %v\n", err)
		os.Exit(1)
	}

	if err = tester.Terminate(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate cluster %v", err)
		os.Exit(1)
	}

	fmt.Println("'aws-k8s-tester etcd delete cluster' success")
}
