package main

import (
	"context"
	"flag"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi"
	"k8s.io/klog/v2"
)

func main() {
	var maxResourceAge time.Duration
	flag.DurationVar(&maxResourceAge, "max-resource-age", time.Hour*3, "Maximum resource age")
	var emitMetrics bool
	flag.BoolVar(&emitMetrics, "emit-metrics", false, "Send metrics to CloudWatch")
	flag.Parse()
	j := eksapi.NewJanitor(maxResourceAge, emitMetrics)
	if err := j.Sweep(context.Background()); err != nil {
		klog.Fatalf("failed to sweep resources: %v", err)
	}
}
