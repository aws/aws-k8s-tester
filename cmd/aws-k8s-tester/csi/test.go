package csi

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

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
	cmd.PersistentFlags().StringVar(&vpcID, "vpc-id", "vpc-0c59620d91b2e1f92", "existing VPC ID to use (provided default VPC ID belongs to aws-k8s-tester test account, leave empty to create a new one)")
	cmd.PersistentFlags().BoolVar(&journalctlLogs, "journalctl-logs", false, "true to get journalctl logs from EC2 instance")

	cmd.AddCommand(
		newTestIntegration(),
	)
	return cmd
}

var (
	terminateOnExit bool
	branchOrPR      string
	timeout         time.Duration
	vpcID           string
	journalctlLogs  bool
)

/*
go install -v ./cmd/aws-k8s-tester

AWS_SHARED_CREDENTIALS_FILE=~/.aws/credentials \
  aws-k8s-tester csi test integration \
  --terminate-on-exit=true \
  --csi=master \
  --timeout=20m
*/

// TODO: use instance role, and get rid of credential mount
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
	cfg.IngressCIDRs = map[int64]string{22: "0.0.0.0/0"}
	cfg.Plugins = []string{
		"update-amazon-linux-2",
		"set-env-aws-cred-AWS_SHARED_CREDENTIALS_FILE",
		"install-go-1.11.2",
		"install-csi-" + branchOrPR,
	}
	cfg.Wait = true
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

	fmt.Println(cfg.SSHCommands())

	var iv ec2config.Instance
	for _, v := range cfg.Instances {
		iv = v
		break
	}

	sh, serr := ssh.New(ssh.Config{
		Logger:        ec.Logger(),
		KeyPath:       cfg.KeyPath,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
		UserName:      cfg.UserName,
	})
	if serr != nil {
		fmt.Fprintf(os.Stderr, "failed to create SSH (%v)\n", err)
		if terminateOnExit {
			ec.Terminate()
		} else {
			fmt.Println(cfg.SSHCommands())
		}
		os.Exit(1)
	}
	defer sh.Close()

	if err = sh.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect SSH (%v)\n", err)
		if terminateOnExit {
			ec.Terminate()
		} else {
			fmt.Println(cfg.SSHCommands())
		}
		os.Exit(1)
	}

	testCmd := fmt.Sprintf(`cd /home/%s/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver && sudo sh -c 'source /home/%s/.bashrc && make test-integration'`, cfg.UserName, cfg.UserName)
	out, err := sh.Run(
		testCmd,
		ssh.WithTimeout(10*time.Minute),
	)
	if err != nil {
		// handle "Process exited with status 2" error
		fmt.Fprintf(os.Stderr, "CSI integration test FAILED (%v, %v)\n", err, reflect.TypeOf(err))
		if terminateOnExit {
			ec.Terminate()
		} else {
			fmt.Println(cfg.SSHCommands())
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
			ec.Terminate()
		} else {
			fmt.Println(cfg.SSHCommands())
		}
		os.Exit(1)
	}

	if journalctlLogs {
		// full journal logs (e.g. disk mounts)
		lg.Info("fetching journal logs")
		journalCmd := "sudo journalctl --no-pager --output=short-precise"
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
		ec.Terminate()
	}
}
