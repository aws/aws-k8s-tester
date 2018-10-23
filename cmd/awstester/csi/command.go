package csi

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/awstester/internal/ec2"
	ec2config "github.com/aws/awstester/internal/ec2/config"
	"github.com/aws/awstester/internal/ec2/config/plugins"
	"github.com/aws/awstester/internal/ssh"
	"github.com/aws/awstester/pkg/fileutil"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewCommand returns a new 'csi' command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csi",
		Short: "CSI commands",
	}
	cmd.AddCommand(
		newTest(),
	)
	return cmd
}

var (
	region         string
	customEndpoint string
)

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test CSI",
	}
	cmd.AddCommand(
		newTestE2E(),
	)
	return cmd
}

/*
tail -f /var/log/cloud-init-output.log

AWS_SHARED_CREDENTIALS_FILE=~/.aws/credentials \
  awstester csi test e2e \
  --terminate-on-exit true \
  --csi master \
  --timeout 10m
*/

func newTestE2E() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "Test CSI e2e without container and Kubernetes",
		Run:   testE2EFunc,
	}
	cmd.PersistentFlags().BoolVar(&terminateOnExit, "terminate-on-exit", true, "true to terminate EC2 instance on test exit")
	cmd.PersistentFlags().StringVar(&csiBranchOrPR, "csi", "master", "CSI branch name or PR number to check out")
	cmd.PersistentFlags().DurationVar(&csiE2ETimeout, "timeout", 10*time.Minute, "CSI e2e test timeout")
	return cmd
}

var terminateOnExit bool
var csiBranchOrPR string
var csiE2ETimeout time.Duration

func testE2EFunc(cmd *cobra.Command, args []string) {
	credEnv := "AWS_SHARED_CREDENTIALS_FILE"
	if os.Getenv(credEnv) == "" || !fileutil.Exist(os.Getenv(credEnv)) {
		fmt.Fprintln(os.Stderr, "no AWS_SHARED_CREDENTIALS_FILE found")
		os.Exit(1)
	}
	if csiE2ETimeout == time.Duration(0) {
		fmt.Fprintf(os.Stderr, "no timeout specified (%q)\n", csiE2ETimeout)
		os.Exit(1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}
	lg.Info(
		"starting CSI e2e tests",
		zap.String("csi", csiBranchOrPR),
		zap.Duration("timeout", csiE2ETimeout),
	)

	cfg := ec2config.NewDefault()
	cfg.Plugins = []string{
		"update-ubuntu",
		"mount-aws-cred-AWS_SHARED_CREDENTIALS_FILE",
		"install-go1.11.1-ubuntu",
		"install-csi-" + csiBranchOrPR,
	}
	ec, err := ec2.NewDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EC2 deployer (%v)\n", err)
		os.Exit(1)
	}
	if err = ec.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EC2 instance (%v)\n", err)
		os.Exit(1)
	}

	fmt.Println(ec.GenerateSSHCommands())

	sh, serr := ssh.New(ssh.Config{
		Logger:   ec.Logger(),
		KeyPath:  cfg.KeyPath,
		Addr:     cfg.Instances[0].PublicIP + ":22",
		UserName: cfg.UserName,
	})
	if serr != nil {
		fmt.Fprintf(os.Stderr, "failed to create SSH (%v)\n", err)
		if terminateOnExit {
			ec.Delete()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}
	if err = sh.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", err)
		if terminateOnExit {
			ec.Delete()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}

	var out []byte

	timer := time.NewTimer(csiE2ETimeout)
ready:
	for {
		select {
		case <-timer.C:
			fmt.Fprintf(os.Stderr, "test timed out (%v)\n", csiE2ETimeout)
			if terminateOnExit {
				ec.Delete()
			} else {
				fmt.Println(ec.GenerateSSHCommands())
			}
			os.Exit(1)

		default:
			out, err = sh.Run("tail -20 /var/log/cloud-init-output.log")
			if err != nil {
				lg.Warn("failed to fetch cloud-init-output.log", zap.Error(err))
				time.Sleep(7 * time.Second)
				continue
			}

			if strings.Contains(string(out), plugins.READY) {
				lg.Info("cloud-init-output.log READY!")
				break ready
			}

			lg.Info("cloud-init-output NOT READY")
			time.Sleep(7 * time.Second)
		}
	}

	out, err = sh.Run("cat /etc/environment")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch /etc/environment (%v)\n", err)
		if terminateOnExit {
			ec.Delete()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}
	env := ""
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		env += line + " "
	}

	testCmd := fmt.Sprintf(`cd /home/ubuntu/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver \
  && sudo sh -c '%s make test-e2e'
`, env)
	out, err = sh.Run(testCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run test (%v)\n", err)
		if terminateOnExit {
			ec.Delete()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}

	testOutput := string(out)
	fmt.Printf("CSI test output:\n\n%s\n\n", testOutput)

	if terminateOnExit {
		ec.Delete()
	} else {
		fmt.Println(ec.GenerateSSHCommands())
	}

	/*
	   expects

	   Ran 1 of 1 Specs in 25.028 seconds
	   SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 0 Skipped
	*/
	if !strings.Contains(testOutput, "SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 0 Skipped") {
		fmt.Fprintln(os.Stderr, "CSI e2e test FAILED")
		os.Exit(1)
	}
}
