//go:build e2e

package quick

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv env.Environment
)

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	testenv.Setup(func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		log.Println("Starting quick test suite...")
		return ctx, nil
	})

	os.Exit(testenv.Run(m))
}
