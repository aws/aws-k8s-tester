package eks

import (
	"fmt"
	"os"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks"

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

	var tester eksdeployer.Tester
	tester, err = eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	err = tester.IsUp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check cluster status %v\n", err)
		os.Exit(1)
	}

	fmt.Println("'awstester eks check cluster' success")
}
