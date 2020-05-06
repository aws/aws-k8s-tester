// Package delete implements "ec2-utils delete" commands.
package delete

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

var (
	path         string
	enablePrompt bool
)

// NewCommand implements "ec2-utils delete" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "delete",
		Short:      "EC2 delete commands",
		SuggestFor: []string{"delet"},
	}
	cmd.PersistentFlags().StringVarP(&path, "path", "p", "", "ec2 test configuration file path")
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.AddCommand(
		newDeleteInstances(),
	)
	return cmd
}

func newDeleteInstances() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instances",
		Short: "Delete EC2 instances",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   deleteInstancesFunc,
	}
	return cmd
}

func deleteInstancesFunc(cmd *cobra.Command, args []string) {
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
		fmt.Printf("'ec2-utils delete instances' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'ec2-utils delete instances' success\n")
}
