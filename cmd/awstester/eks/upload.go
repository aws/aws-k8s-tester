package eks

import (
	"fmt"
	"os"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	"github.com/aws/awstester/internal/eks"
	"github.com/aws/awstester/pkg/fileutil"

	"github.com/spf13/cobra"
)

func newUpload() *cobra.Command {
	ac := &cobra.Command{
		Use:   "upload <subcommand>",
		Short: "Upload commands",
	}
	ac.AddCommand(newUploadS3())
	return ac
}

func newUploadS3() *cobra.Command {
	return &cobra.Command{
		Use:   "s3 [local file path] [remote S3 path]",
		Short: "Upload a file to S3",
		Run:   uploadS3Func,
	}
}

func uploadS3Func(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "expected 2 arguments, got %v\n", args)
		os.Exit(1)
	}

	if !fileutil.Exist(path) {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q\n", path)
		os.Exit(1)
	}

	cfg, err := eksconfig.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	var dp eksdeployer.Interface
	dp, err = eks.NewEKSDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	from, to := args[0], args[1]
	if err = dp.DumpClusterLogs(from, to); err != nil {
		fmt.Fprintf(os.Stderr, "failed to upload from %q to %q (%v)\n", from, to, err)
		os.Exit(1)
	}

	fmt.Println("'awstester eks upload s3' success")
}
