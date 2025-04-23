//go:build e2e

package dra

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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg).WithContext(ctx)
	os.Exit(testenv.Run(m))
}
