package storage

import (
	"math/rand"
	"os"
	"testing"
	"time"

	_ "github.com/aws/aws-k8s-tester/k8s-tester/csi-ebs"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	//5 Minutes is slow test
	config.DefaultReporterConfig.SlowSpecThreshold = 300
	RegisterFailHandler(Fail)
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestStorage(t *testing.T) {
	RunSpecs(t, "Storage Suite")
}
