package e2e

import (
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs"
	_ "github.com/aws/aws-k8s-tester/k8s-tester/falco"
	_ "github.com/aws/aws-k8s-tester/k8s-tester/secrets"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	config.DefaultReporterConfig.SlowSpecThreshold = 300
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestSuites(t *testing.T) {
	RunSpecs(t, "e2e Suite")
}
