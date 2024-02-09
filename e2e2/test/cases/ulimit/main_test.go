package ulimit

import (
	"os"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"testing"
)

var (
	testenv env.Environment
)

func TestMain(m *testing.M) {
	testenv = env.New()
	os.Exit(testenv.Run(m))
}
