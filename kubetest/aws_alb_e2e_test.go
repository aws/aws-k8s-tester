package kubetest_test

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
	eksplugin "github.com/aws/awstester/kubetest/eks"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/test-infra/kubetest/process"

	// remove "internal" package imports
	// when it gets contributed to upstream
	"github.com/aws/awstester/internal/eks"
)

// http://onsi.github.io/ginkgo/#the-ginkgo-cli
var (
	ginkgoTimeout = flag.Duration("ginkgo-command-timeout", 10*time.Hour, "timeout for test commands")
	ginkgoVerbose = flag.Bool("ginkgo-verbose", true, "'true' to enable verbose in Ginkgo")
)

var cfg = eksconfig.NewDefault()

func TestMain(m *testing.M) {
	flag.Parse()

	cfg.UpdateFromEnvs()
	cfg.Sync()

	// auto-generate test configuration file
	// so that tester does not need write one for kubetest
	f, err := ioutil.TempFile(os.TempDir(), "awstester")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to save temporary file %v\n", err)
		os.Exit(1)
	}
	outputPath := f.Name()
	f.Close()
	cfg.ConfigPath, err = filepath.Abs(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to expand output path %v\n", err)
		os.Exit(1)
	}

	if err = cfg.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to fsync %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestAWSTesterEKS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "awstester eks ALB Ingress Controller e2e tests")
}

var (
	kp      eksdeployer.Interface
	control *process.Control
)

var _ = BeforeSuite(func() {
	control = process.NewControl(
		*ginkgoTimeout,
		time.NewTimer(3*time.Hour),
		time.NewTimer(3*time.Hour),
		*ginkgoVerbose,
	)

	var err error
	if cfg.Embedded {
		kp, err = eks.NewEKSDeployer(cfg)
	} else {
		kp, err = eksplugin.New(cfg)
	}
	Expect(err).ShouldNot(HaveOccurred())

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	donec := make(chan struct{})
	go func() {
		select {
		case <-donec:
			fmt.Fprintf(os.Stderr, "finished 'Up'")
		case sig := <-notifier:
			fmt.Fprintf(os.Stderr, "received signal %q in BeforeSuite\n", sig)
			kp.Stop()
			var derr error
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

var _ = AfterSuite(func() {
	if !cfg.Embedded {
		// reload updated kubeconfig
		var err error
		cfg, err = eksconfig.Load(cfg.ConfigPath)
		Expect(err).ShouldNot(HaveOccurred())
	}

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

var _ = Describe("EKS with ALB Ingress Controller on worker nodes", func() {
	Context("Correctness of EKS cluster", func() {
		It("EKS expects worker nodes", func() {
			if !cfg.Embedded {
				// reload updated kubeconfig
				var err error
				cfg, err = eksconfig.Load(cfg.ConfigPath)
				Expect(err).ShouldNot(HaveOccurred())
			}

			// TODO: run Kubernetes upstream e2e tests
			co, err := control.Output(exec.Command(
				"kubectl",
				"--kubeconfig="+cfg.KubeConfigPath,
				"get",
				"nodes",
			))
			Expect(err).ShouldNot(HaveOccurred())
			nn := strings.Count(string(co), "Ready")
			Expect(nn).To(Equal(int(cfg.WorkderNodeASGMax)))
		})
	})

	Context("Correctness of ALB Ingress Controller on worker nodes", func() {
		It("ALB Ingress Controller expects Ingress rules", func() {
			err := kp.TestCorrectness()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Scalability of ALB Ingress Controller on worker nodes", func() {
		fmt.Fprintf(
			os.Stdout,
			"ALBIngressController.EnableScalabilityTest %v\n",
			cfg.ALBIngressController.EnableScalabilityTest,
		)

		if cfg.ALBIngressController.EnableScalabilityTest {
			It("ALB Ingress Controller expects to handle concurrent clients with expected QPS", func() {
				err := kp.TestQPS()
				Expect(err).ShouldNot(HaveOccurred())
			})
		}

		// enough time to process metrics
		// and to not overload ingress controller
		time.Sleep(3 * time.Second)

		It("ALB Ingress Controller expects to serve '/metrics'", func() {
			err := kp.TestMetrics()
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
