package integration_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
)

/*
RUN_AWS_TESTS=1 go test -v -timeout 2h -run TestEC2SSH
*/
func TestEC2SSH(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := ec2config.NewDefault()
	cfg.Wait = true
	cfg.Plugins = []string{
		"update-amazon-linux-2",
		"install-go1.11.1",

		// "install-etcd-3.1.12",
		"install-etcd-master",
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
	out, err = sh.Run(
		"curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("availability-zone:", string(out))

	out, err = sh.Run(
		"source /etc/environment && go version",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Error(err)
	}
	if string(out) != "go version go1.11.1 linux/amd64\n" {
		t.Fatalf("unexpected go version %q", string(out))
	}

	time.Sleep(5 * time.Minute)
}
