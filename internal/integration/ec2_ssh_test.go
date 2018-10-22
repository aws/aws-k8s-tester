package integration_test

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/aws/awstester/internal/ec2"
	ec2config "github.com/aws/awstester/internal/ec2/config"
	"github.com/aws/awstester/internal/ssh"
)

func TestEC2SSH(t *testing.T) {
	if os.Getenv("RUN_AWS_UNIT_TESTS") != "1" {
		t.Skip()
	}

	cfg := ec2config.NewDefault()

	// tail -f /var/log/cloud-init-output.log
	cfg.Plugins = []string{"update-ubuntu", "go1.11.1-ubuntu"}

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

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("received:", (<-notifier).String())
	signal.Stop(notifier)

	var out []byte
	out, err = sh.Run("printenv")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("printenv", string(out))

	out, err = sh.Run("cat /var/log/cloud-init-output.log")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("cloud-init-output.log", string(out))

	out, err = sh.Run("cat /etc/environment")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("/etc/environment", string(out))

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
}
