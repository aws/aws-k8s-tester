package ec2

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func newDelete() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <subcommand>",
		Short: "Delete commands",
	}
	cmd.AddCommand(newDeleteCluster())
	return cmd
}

func newDeleteCluster() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Delete EC2 cluster",
		Run:   deleteClusterFunc,
	}
}

func deleteClusterFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := ec2config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	println()
	fmt.Println(string(txt))
	println()

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to delete EC2 resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's delete!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'delete' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	tester, err := ec2.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ec2 deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Down(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester ec2 delete cluster' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester ec2 delete cluster' successs\n")
}
