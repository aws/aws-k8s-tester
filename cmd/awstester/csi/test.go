package csi

import (
	"fmt"
	"os"
	"reflect"
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

func newTest() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run CSI tests",
	}

	cmd.PersistentFlags().BoolVar(&terminateOnExit, "terminate-on-exit", true, "true to terminate EC2 instance on test exit")
	cmd.PersistentFlags().StringVar(&branchOrPR, "csi", "master", "CSI branch name or PR number to check out")
	cmd.PersistentFlags().DurationVar(&timeout, "timeout", 20*time.Minute, "e2e test timeout")
	cmd.PersistentFlags().StringVar(&vpcID, "vpc-id", "vpc-0c59620d91b2e1f92", "existing VPC ID to use (provided default VPC ID belongs to awstester test account, leave empty to create a new one)")
	cmd.PersistentFlags().BoolVar(&journalctlLogs, "journalctl-logs", false, "true to get journalctl logs from EC2 instance")

	cmd.AddCommand(
		newTestIntegration(),
	)
	return cmd
}

var terminateOnExit bool
var branchOrPR string
var timeout time.Duration
var vpcID string
var journalctlLogs bool

/*
go install -v ./cmd/awstester

AWS_SHARED_CREDENTIALS_FILE=~/.aws/credentials \
  awstester csi test integration \
  --terminate-on-exit=true \
  --csi=master \
  --timeout=20m
*/

func newTestIntegration() *cobra.Command {
	return &cobra.Command{
		Use:   "integration",
		Short: "Run CSI integration tests without container and Kubernetes",
		Run:   testIntegrationFunc,
	}
}

func testIntegrationFunc(cmd *cobra.Command, args []string) {
	credEnv := "AWS_SHARED_CREDENTIALS_FILE"
	if os.Getenv(credEnv) == "" || !fileutil.Exist(os.Getenv(credEnv)) {
		fmt.Fprintln(os.Stderr, "no AWS_SHARED_CREDENTIALS_FILE found")
		os.Exit(1)
	}
	if timeout == time.Duration(0) {
		fmt.Fprintf(os.Stderr, "no timeout specified (%q)\n", timeout)
		os.Exit(1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}
	lg.Info(
		"starting CSI integration tests",
		zap.String("csi", branchOrPR),
		zap.Duration("timeout", timeout),
	)

	cfg := ec2config.NewDefault()
	cfg.UploadTesterLogs = false
	cfg.VPCID = vpcID
	cfg.Plugins = []string{
		"update-ubuntu",
		"mount-aws-cred-AWS_SHARED_CREDENTIALS_FILE",
		"install-go1.11.1",
		"install-csi-" + branchOrPR,
	}
	var ec ec2.Deployer
	ec, err = ec2.NewDeployer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EC2 deployer (%v)\n", err)
		os.Exit(1)
	}
	if err = ec.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EC2 instance (%v)\n", err)
		os.Exit(1)
	}

	deleteFunc := func() {
		os.RemoveAll(cfg.KeyPath)
		lg.Warn("removed private key", zap.String("path", cfg.KeyPath))
		ec.Delete()
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
			deleteFunc()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}
	defer sh.Close()

	if err = sh.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", err)
		if terminateOnExit {
			deleteFunc()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}

	var out []byte

	timer := time.NewTimer(timeout)

ready:
	for {
		select {
		case <-timer.C:
			fmt.Fprintf(os.Stderr, "test timed out (%v)\n", timeout)
			if terminateOnExit {
				deleteFunc()
			} else {
				fmt.Println(ec.GenerateSSHCommands())
			}
			os.Exit(1)

		default:
			out, err = sh.Run(
				"tail -10 /var/log/cloud-init-output.log",
				ssh.WithRetry(100, 5*time.Second),
				ssh.WithTimeout(30*time.Second),
			)
			if err != nil {
				lg.Warn("failed to fetch cloud-init-output.log", zap.Error(err))
				sh.Close()
				if serr := sh.Connect(); serr != nil {
					fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", serr)
					if terminateOnExit {
						deleteFunc()
					} else {
						fmt.Println(ec.GenerateSSHCommands())
					}
					os.Exit(1)
				}
				time.Sleep(7 * time.Second)
				continue
			}

			fmt.Printf("\n\n%s\n\n", string(out))

			if strings.Contains(string(out), plugins.READY) {
				lg.Info("cloud-init-output.log READY!")
				break ready
			}

			lg.Info("cloud-init-output NOT READY")
			time.Sleep(7 * time.Second)
		}
	}

	out, err = sh.Run(
		"cat /etc/environment",
		ssh.WithRetry(30, 5*time.Second),
		ssh.WithTimeout(30*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read /etc/environment (%v)\n", err)
		if terminateOnExit {
			deleteFunc()
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
  && sudo sh -c '%s make test-integration'
`, env)
	out, err = sh.Run(
		testCmd,
		ssh.WithTimeout(10*time.Minute),
	)
	if err != nil {
		// handle "Process exited with status 2" error
		fmt.Fprintf(os.Stderr, "CSI integration test FAILED (%v, %v)\n", err, reflect.TypeOf(err))
		if terminateOnExit {
			deleteFunc()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}

	testOutput := string(out)
	fmt.Printf("CSI test output:\n\n%s\n\n", testOutput)

	/*
	   expects

	   Ran 1 of 1 Specs in 25.028 seconds
	   SUCCESS! -- 1 Passed | 0 Failed | 0 Pending | 0 Skipped
	*/
	if !strings.Contains(testOutput, "1 Passed") {
		fmt.Fprintln(os.Stderr, "CSI integration test FAILED")
		if terminateOnExit {
			deleteFunc()
		} else {
			fmt.Println(ec.GenerateSSHCommands())
		}
		os.Exit(1)
	}

	if journalctlLogs {
		// full journal logs (e.g. disk mounts)
		lg.Info("fetching journal logs")
		journalCmd := "sudo journalctl --output=short-precise"
		out, err = sh.Run(journalCmd)
		if err != nil {
			lg.Warn(
				"failed to run journalctl",
				zap.String("cmd", journalCmd),
				zap.Error(err),
			)
		} else {
			fmt.Printf("journalctl logs:\n\n%s\n\n", string(out))
		}
	}

	if terminateOnExit {
		deleteFunc()
	}
}
