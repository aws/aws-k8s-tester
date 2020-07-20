// Package createrepo implements "ecr-utils create-repo" commands.
package createrepo

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	pkg_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	enablePrompt     bool
	logLevel         string
	partition        string
	repoAccountID    string
	repoName         string
	regions          []string
	imgScanOnPush    bool
	imgTagMutability string
	policyFilePath   string
	setPolicyForce   bool
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "ecr-utils create-repo" command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-repo",
		Short: "ecr-utils create-repo commands",

		Run: createFunc,
	}
	cmd.PersistentFlags().BoolVarP(&enablePrompt, "enable-prompt", "e", true, "'true' to enable prompt mode")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error, dpanic, panic, fatal)")
	cmd.PersistentFlags().StringVar(&partition, "partition", "aws", "AWS partition")
	cmd.PersistentFlags().StringVar(&repoAccountID, "repo-account-id", "", "AWS repository account ID")
	cmd.PersistentFlags().StringVar(&repoName, "repo-name", "", "AWS ECR repository name")
	cmd.PersistentFlags().StringSliceVar(&regions, "regions", nil, "AWS regions to create repository; if empty create for all available regions")
	cmd.PersistentFlags().BoolVar(&imgScanOnPush, "image-scan-on-push", false, "true to scan images on push")
	cmd.PersistentFlags().StringVar(&imgTagMutability, "image-tag-mutability", ecr.ImageTagMutabilityMutable, "MUTABLE to allow tag overwrites")
	cmd.PersistentFlags().StringVar(&policyFilePath, "policy-file-path", "", "AWS ECR policy JSON file path")
	cmd.PersistentFlags().BoolVar(&setPolicyForce, "set-policy-force", false, "true to force-write ECR repository policy")
	return cmd
}

func createFunc(cmd *cobra.Command, args []string) {
	defaultRegion := ""
	if len(regions) == 0 {
		rm, err := pkg_aws.Regions(partition)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to ger regions for partition %q (%v)\n", partition, err)
			os.Exit(1)
		}
		for rv := range rm {
			regions = append(regions, rv)
		}
		sort.Strings(regions)
		if _, ok := rm["us-west-2"]; ok {
			defaultRegion = "us-west-2"
		}
	}
	if defaultRegion == "" {
		defaultRegion = regions[0]
	}

	lcfg := logutil.GetDefaultZapLoggerConfig()
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(logLevel))
	lg, err := lcfg.Build()
	if err != nil {
		panic(err)
	}
	ss, stsOutput, _, err := pkg_aws.New(&pkg_aws.Config{
		Logger:        lg,
		DebugAPICalls: logLevel == "debug",
		Partition:     partition,
		Region:        defaultRegion,
	})
	if stsOutput == nil || err != nil {
		lg.Fatal("failed to create AWS session and get sts caller identity", zap.Error(err))
	}

	roleARN := aws.StringValue(stsOutput.Arn)
	fmt.Fprintf(os.Stderr, "\nAccount: %q\n", aws.StringValue(stsOutput.Account))
	fmt.Fprintf(os.Stderr, "Role Arn: %q\n", roleARN)
	fmt.Fprintf(os.Stderr, "UserId: %q\n", aws.StringValue(stsOutput.UserId))
	fmt.Fprintf(os.Stderr, "\nRepository Name: %q\n", repoName)
	fmt.Fprintf(os.Stderr, "Regions: %q\n\n", regions)

	if repoAccountID == "" {
		lg.Fatal("empty repo account ID")
	}
	if repoAccountID != aws.StringValue(stsOutput.Account) {
		lg.Fatal("unexpected repo account ID", zap.String("expected", repoAccountID), zap.String("got", aws.StringValue(stsOutput.Account)))
	}
	if repoName == "" {
		lg.Fatal("empty repo name")
	}

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to create ECR resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's create!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'create-repo' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	for _, region := range regions {
		fmt.Fprintf(os.Stderr, "\n\n********************\n")
		ecrSvc := ecr.New(ss, aws.NewConfig().WithRegion(region))
		if policyFilePath != "" && !fileutil.Exist(policyFilePath) {
			lg.Fatal("ECR repository policy file not found", zap.String("policy-file-path", policyFilePath))
		}
		policyTxt := ""
		if fileutil.Exist(policyFilePath) {
			d, err := ioutil.ReadFile(policyFilePath)
			if err != nil {
				lg.Fatal("failed to read policy file", zap.Error(err))
			}
			policyTxt = string(d)
		}
		repoURI, err := pkg_ecr.Create(
			lg,
			ecrSvc,
			repoAccountID,
			region,
			repoName,
			imgScanOnPush,
			imgTagMutability,
			policyTxt,
			setPolicyForce,
		)
		if err != nil {
			lg.Warn("failed to create", zap.Error(err))
		} else {
			fmt.Fprintf(os.Stderr, "ECR created %q (%q)\n", repoURI, region)
		}
	}
}
