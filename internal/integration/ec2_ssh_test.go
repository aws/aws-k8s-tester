package integration_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/awstester/internal/ec2"
	ec2config "github.com/aws/awstester/internal/ec2/config"
	"github.com/aws/awstester/internal/ec2/config/plugins"
	"github.com/aws/awstester/internal/ssh"
)

/*
RUN_AWS_UNIT_TESTS=1 go test -v -run TestEC2SSHWithCSI
RUN_AWS_UNIT_TESTS=1 AWS_SHARED_CREDENTIALS_FILE=~/.aws/credentials go test -v -timeout 2h -run TestEC2SSHWithCSI
*/
func TestEC2SSHWithCSI(t *testing.T) {
	if os.Getenv("RUN_AWS_UNIT_TESTS") != "1" {
		t.Skip()
	}

	cfg := ec2config.NewDefault()

	// tail -f /var/log/cloud-init-output.log
	cfg.Plugins = []string{
		"update-ubuntu",
		"mount-aws-cred",
		"install-go1.11.1-ubuntu",
		"install-csi-master",
	}

	ec, err := ec2.NewDeployer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = ec.Create(); err != nil {
		t.Fatal(err)
	}
	defer ec.Delete()

	fmt.Println(ec.GenerateSSHCommands())

	sh, serr := ssh.New(ssh.Config{
		Logger:   ec.Logger(),
		KeyPath:  cfg.KeyPath,
		Addr:     cfg.Instances[0].PublicIP + ":22",
		UserName: cfg.UserName,
	})
	if serr != nil {
		t.Fatal(err)
	}
	if err = sh.Connect(); err != nil {
		t.Fatal(err)
	}

	var out []byte

	timer := time.NewTimer(10 * time.Minute)
ready:
	for {
		select {
		case <-timer.C:
			t.Fatal("not ready")

		default:
			out, err = sh.Run("cat /var/log/cloud-init-output.log")
			if err != nil {
				t.Log(err)
				time.Sleep(10 * time.Second)
				continue
			}

			if strings.Contains(string(out), plugins.READY) {
				fmt.Println("cloud-init-output.log READY:", string(out))
				break ready
			}

			fmt.Println("cloud-init-output.log:", string(out))
			time.Sleep(10 * time.Second)
		}
	}

	out, err = sh.Run("cat /etc/environment")
	if err != nil {
		t.Error(err)
	}
	env := ""
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		env += line + " "
	}

	out, err = sh.Run("source /etc/environment && go version")
	if err != nil {
		t.Error(err)
	}
	if string(out) != "go version go1.11.1 linux/amd64\n" {
		t.Fatalf("unexpected go version %q", string(out))
	}

	out, err = sh.Run("curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("availability-zone:", string(out))

	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		cmd := fmt.Sprintf(`cd /home/ubuntu/go/src/github.com/kubernetes-sigs/aws-ebs-csi-driver \
  && sudo sh -c '%s make test-e2e'
`, env)
		out, err = sh.Run(cmd)
		if err != nil {
			t.Error(err)
		}
		fmt.Println("CSI test:", string(out))
	}
}
