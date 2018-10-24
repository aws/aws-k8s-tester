package kubetest_test

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/awstester/eksdeployer"
	eksplugin "github.com/aws/awstester/kubetest/eks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// remove "internal" package imports
	// when it gets contributed to upstream
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
	RunSpecs(t, "awstester eks ALB Ingress Controller e2e tests")
}

var kp eksdeployer.Interface

// to use embedded eks
// kp, err = eks.NewEKSDeployer(cfg)
var _ = BeforeSuite(func() {
	var err error
	kp, err = eksplugin.New(*timeout, *verbose)
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
			kp.Stop()
			cfg, derr := kp.LoadConfig()
			Expect(derr).ShouldNot(HaveOccurred())
			if cfg.Down {
				derr = kp.Down()
			}
			signal.Stop(notifier)
			<-donec // wait until 'Up' complete
			fmt.Fprintf(os.Stderr, "shut down cluster with %q in BeforeSuite (down error %v)\n", sig, derr)
			os.Exit(1)
		}
	}()

	err = kp.Up()
	close(donec)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = Describe("EKS with ALB Ingress Controller on worker nodes", func() {
	Context("Correctness of ALB Ingress Controller on worker nodes", func() {
		It("ALB Ingress Controller expects Ingress rules", func() {
			err := kp.TestALBCorrectness()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Scalability of ALB Ingress Controller on worker nodes", func() {
		cfg, derr := kp.LoadConfig()
		Expect(derr).ShouldNot(HaveOccurred())
		if cfg.ALBIngressController.TestScalability {
			It("ALB Ingress Controller expects to handle concurrent clients with expected QPS", func() {
				err := kp.TestALBQPS()
				Expect(err).ShouldNot(HaveOccurred())
			})

			// enough time to process metrics
			// and to not overload ingress controller
			time.Sleep(3 * time.Second)
		}

		It("ALB Ingress Controller expects to serve '/metrics'", func() {
			err := kp.TestALBMetrics()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})

var _ = AfterSuite(func() {
	// reload updated kubeconfig
	cfg, err := kp.LoadConfig()
	Expect(err).ShouldNot(HaveOccurred())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	interrupted := false
	if cfg.Down && cfg.WaitBeforeDown > 0 {
		fmt.Fprintf(os.Stderr, "\n\n\n%v: waiting %v before cluster tear down (if not interrupted)...\n\n\n", time.Now().UTC(), cfg.WaitBeforeDown)
		select {
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received signal %q in AfterSuite\n", sig)
			var derr error
			if cfg.Down {
				derr = kp.Down()
			}
			signal.Stop(notifier)
			fmt.Fprintf(os.Stderr, "shut down cluster with %q (with %v) in AfterSuite\n", sig, derr)
			interrupted = true
		case <-time.After(cfg.WaitBeforeDown):
			// Note: takes about 5~10 minutes for ALB access logs be available in S3
		}
	}
	if interrupted {
		return
	}

	if cfg.Down {
		err := kp.Down()
		Expect(err).ShouldNot(HaveOccurred())
	}
})
