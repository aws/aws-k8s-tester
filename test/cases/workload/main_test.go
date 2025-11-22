//go:build e2e

package workload

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	defaultWorkloadTestTimeout = 10 * time.Minute
)

var (
	testenv               env.Environment
	workloadTestCommand   *string
	workloadTestImage     *string
	workloadTestName      *string
	workloadTestResources *string
	workloadTestTimeout   *time.Duration
)

func TestMain(m *testing.M) {
	workloadTestCommand = flag.String("workloadTestCommand", "", "command for workload test")
	workloadTestImage = flag.String("workloadTestImage", "", "image for workload test")
	workloadTestName = flag.String("workloadTestName", "workload-test", "name for workload test")
	workloadTestResources = flag.String("workloadTestResources", "", "JSON map of resources for workload test (e.g., '{\"nvidia.com/gpu\": \"1\"}')")
	workloadTestTimeout = flag.Duration("workloadTestTimeout", defaultWorkloadTestTimeout, fmt.Sprintf("timeout for workload test (default: %s)", defaultWorkloadTestTimeout))
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	testenv.Setup(func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		log.Println("Starting workload test suite...")
		return ctx, nil
	})

	os.Exit(testenv.Run(m))
}
