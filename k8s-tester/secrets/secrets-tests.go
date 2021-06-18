package secrets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/aws/aws-k8s-tester/utils/log"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var (
	kubeconfigPath string
)

var _ = ginkgo.Describe("[security-secrets]", func() {
	if home := os.Getenv("HOME"); home != "" {
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	} else {
		kubeconfigPath = client.DefaultKubectlPath()
	}
	lg, logWriter, _, _ := log.NewWithStderrWriter(log.DefaultLogLevel, []string{"stderr"})
	_ = zap.ReplaceGlobals(lg)
	cli, _ := client.New(&client.Config{
		Logger:         lg,
		KubeconfigPath: kubeconfigPath,
	})
	cfg := NewDefault()
	cfg.LogWriter = logWriter
	cfg.Logger = lg
	cfg.Enable = true
	cfg.Client = cli
	ts := New(cfg)
	ginkgo.BeforeEach(func() {
		ginkgo.By(fmt.Sprintf("Add any before functions here."))
	})
	ginkgo.AfterEach(func() {
		ginkgo.By(fmt.Sprintf("Cleaning up K8s resources from tests"))
		ts.Delete()
	})
	ginkgo.It("should Install Secrets Helm Chart without Error", func() {
		ginkgo.By("It should run the Secrets test without error")
		Expect(ts.Apply()).Should(Succeed())
	})
})
