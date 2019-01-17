package kubetest_test

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/ekstester"
	"github.com/aws/aws-k8s-tester/kubetest/eks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// http://onsi.github.io/ginkgo/#the-ginkgo-cli
var (
	timeout = flag.Duration("ginkgo-timeout", 10*time.Hour, "timeout for test commands")
	verbose = flag.Bool("ginkgo-verbose", true, "'true' to enable verbose in Ginkgo")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestAWSTesterEKS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "aws-k8s-tester eks ALB Ingress Controller e2e tests")
}

var tester ekstester.Tester

var _ = BeforeSuite(func() {
	var err error
	tester, err = eks.NewTester(*timeout, *verbose)
	Expect(err).ShouldNot(HaveOccurred())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	donec := make(chan struct{})
	go func() {
		select {
		case <-donec:
			fmt.Fprintf(os.Stderr, "finished 'Up'\n")
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received signal %q in BeforeSuite\n", sig)
			tester.Stop()
			cfg, derr := tester.LoadConfig()
			Expect(derr).ShouldNot(HaveOccurred())
			if cfg.Down {
				derr = tester.Down()
			}
			signal.Stop(notifier)
			<-donec // wait until 'Up' complete
			fmt.Fprintf(os.Stderr, "shut down cluster with %q in BeforeSuite (down error %v)\n", sig, derr)
			os.Exit(1)
		}
	}()

	err = tester.Up()
	close(donec)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = Describe("EKS with ALB Ingress Controller on worker nodes", func() {
	Context("Correctness of ALB Ingress Controller on worker nodes", func() {
		It("ALB Ingress Controller expects Ingress rules", func() {
			err := tester.TestALBCorrectness()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Scalability of ALB Ingress Controller on worker nodes", func() {
		if tester == nil {
			// ginkgo/internal/suite.(*Suite).PushContainerNode
			return
		}

		cfg, derr := tester.LoadConfig()
		Expect(derr).ShouldNot(HaveOccurred())
		if cfg.ALBIngressController.TestScalability {
			It("ALB Ingress Controller expects to handle concurrent clients with expected QPS", func() {
				err := tester.TestALBQPS()
				Expect(err).ShouldNot(HaveOccurred())
			})

			// enough time to process metrics
			// and to not overload ingress controller
			time.Sleep(3 * time.Second)
		}

		It("ALB Ingress Controller expects to serve '/metrics'", func() {
			err := tester.TestALBMetrics()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

var _ = AfterSuite(func() {
	// reload updated kubeconfig
	cfg, err := tester.LoadConfig()
	Expect(err).ShouldNot(HaveOccurred())

	if cfg.Down {
		err := tester.Down()
		Expect(err).ShouldNot(HaveOccurred())
	}
})
