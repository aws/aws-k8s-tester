package eks

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/spf13/cobra"
)

func newCheck() *cobra.Command {
	ac := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "Check EKS resources",
	}
	ac.AddCommand(
		newCheckCluster(),
	)
	return ac
}

func newCheckCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Check EKS cluster status",
		Run:   checkClusterFunc,
	}
}

func checkClusterFunc(cmd *cobra.Command, args []string) {
	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	var up bool
	up, err = tester.IsUp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check cluster status %v\n", err)
		os.Exit(1)
	}
	if !up {
		fmt.Fprintln(os.Stderr, "failed to check cluster status: not up")
		os.Exit(1)
	}
	fmt.Println("'aws-k8s-tester eks check cluster' success")
}
