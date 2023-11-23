package main

import (
	"context"
	"flag"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployer/eksapi"
	"k8s.io/klog/v2"
)

func main() {
	var maxResourceAge time.Duration
	flag.DurationVar(&maxResourceAge, "max-resource-age", time.Hour*3, "Maximum resource age")
	flag.Parse()
	j := eksapi.NewJanitor(maxResourceAge)
	if err := j.Sweep(context.Background()); err != nil {
		klog.Fatalf("failed to sweep resources: %v", err)
	}
}
