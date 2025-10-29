//go:build e2e

package workload

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv             env.Environment
	workloadTestCommand *string
	workloadTestImage   *string
	workloadTestName    *string
)

func TestMain(m *testing.M) {
	workloadTestCommand = flag.String("workloadTestCommand", "", "command for workload test")
	workloadTestImage = flag.String("workloadTestImage", "", "image for workload test")
	workloadTestName = flag.String("workloadTestName", "workload-test", "name for workload test")
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
