// etcd-utils is a set of etcd utilities commands.
package main

import (
	"fmt"
	"os"

	k8s "github.com/aws/aws-k8s-tester/cmd/etcd-utils/k8s"
	"github.com/aws/aws-k8s-tester/cmd/etcd-utils/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:        "etcd-utils",
	Short:      "etcd utils CLI",
	SuggestFor: []string{"etcdutils"},
}

func init() {
	cobra.EnablePrefixMatching = true
}

func init() {
	rootCmd.AddCommand(
		k8s.NewCommand(),
		version.NewCommand(),
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "etcd-utils failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
