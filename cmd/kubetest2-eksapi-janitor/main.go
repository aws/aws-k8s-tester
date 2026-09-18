package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/internal/deployers/eksapi"
)

func main() {
	var maxResourceAge time.Duration
	flag.DurationVar(&maxResourceAge, "max-resource-age", time.Hour*3, "Maximum resource age")
	var workers int
	flag.IntVar(&workers, "workers", 1, "number of workers to processes resources in parallel")
	var stackStatus string
	flag.StringVar(&stackStatus, "stack-status", "", "only process stacks with a specific status")
	var emitMetrics bool
	flag.BoolVar(&emitMetrics, "emit-metrics", false, "Send metrics to CloudWatch")
	var regions string
	flag.StringVar(&regions, "regions", "", "comma-separated regions to sweep; when empty, all regions enabled for the account are swept")
	flag.Parse()
	var regionsList []string
	for _, r := range strings.Split(regions, ",") {
		if r = strings.TrimSpace(r); r != "" {
			regionsList = append(regionsList, r)
		}
	}
	j := eksapi.NewJanitor(maxResourceAge, emitMetrics, workers, stackStatus, regionsList)
	if err := j.Sweep(context.Background()); err != nil {
		slog.Error("failed to sweep resources", "error", err)
		os.Exit(1)
	}
}
