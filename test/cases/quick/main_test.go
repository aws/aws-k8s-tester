//go:build e2e

package quick

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
)

var (
	testenv env.Environment
)

func TestMain(m *testing.M) {
	testenv = env.New()
	os.Exit(testenv.Run(m))
}
