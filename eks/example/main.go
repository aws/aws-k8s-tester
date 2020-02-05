// example shows how to use "eks" package.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
)

func main() {
	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		panic(err)
	}

	os.RemoveAll(cfg.ConfigPath)
	os.RemoveAll(cfg.KubeConfigPath)

	cfg.AddOnNLBHelloWorld.Enable = true
	cfg.AddOnALB2048.Enable = true
	cfg.AddOnJobPerl.Enable = true
	cfg.AddOnJobEcho.Enable = true
	cfg.AddOnSecrets.Enable = true

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
	fmt.Println("received signal:", <-tch)

	if derr := ts.Down(); derr != nil {
		fmt.Println("Down done:", derr)
	}
}
