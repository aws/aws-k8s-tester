package eks

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/spf13/cobra"
)

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <subcommand>",
		Short: "Test commands",
	}
	cmd.AddCommand(
		newTestExample(),
	)
	return cmd
}

func newTestExample() *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "Test with example Pods",
		Run:   testExamples,
	}
}

func testExamples(cmd *cobra.Command, args []string) {
	if path != "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not expected")
		os.Exit(1)
	}
	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		panic(err)
	}
	os.RemoveAll(cfg.ConfigPath)
	os.RemoveAll(cfg.KubeConfigPath)

	println()
	fmt.Println("ConfigPath:", cfg.ConfigPath)
	fmt.Println("KubeConfigPath:", cfg.KubeConfigPath)
	println()

	ts, err := eks.New(cfg)
	if err != nil {
		panic(err)
	}

	err = ts.Up()
	if err != nil {
		panic(err)
	}
	fmt.Println("Up done:", err)

	tch := make(chan os.Signal)
	signal.Notify(tch, syscall.SIGTERM, syscall.SIGINT)

	fmt.Println("waiting for", syscall.SIGTERM, syscall.SIGINT)
	fmt.Println("received signal:", <-tch)

	if derr := ts.Down(); derr != nil {
		fmt.Println("Down done:", derr)
	}
}
