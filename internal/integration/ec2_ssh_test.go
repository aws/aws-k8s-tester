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
	cfg.InitScript = `#!/usr/bin/env bash

echo "Hello World!" > /home/ubuntu/sample.txt
`

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
		UserName: "ubuntu",
	})
	if serr != nil {
		t.Fatal(err)
	}
	if err = sh.Connect(); err != nil {
		t.Fatal(err)
	}
	var out []byte
	out, err = sh.Run("printenv")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("printenv:", string(out))

	out, err = sh.Run("cat /home/ubuntu/sample.txt")
	if err != nil {
		t.Error(err)
	}
	if string(out) != "Hello World!" {
		t.Fatalf("unexpected output %q", string(out))
	}

	out, err = sh.Run("curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone")
	if err != nil {
		t.Error(err)
	}
	fmt.Println("availability-zone:", string(out))

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("received:", (<-notifier).String())
	signal.Stop(notifier)
}
