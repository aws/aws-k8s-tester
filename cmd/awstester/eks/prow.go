package eks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/internal/eks/alb"
	"github.com/aws/awstester/internal/prow/status"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newProw() *cobra.Command {
	ac := &cobra.Command{
		Use:   "prow <subcommand>",
		Short: "Prow commands",
	}
	ac.AddCommand(
		newProwStatus(),
		newProwALB(),
	)
	return ac
}

/*
http://localhost:32010/eks-test-status-upstream
http://localhost:32010/eks-test-status-upstream-refresh
*/
func newProwStatus() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check Kubernetes upstream test status",
		Run:   prowStatusFunc,
	}
	cmd.PersistentFlags().StringVar(&prowStatusPort, "port", ":32010", "port to serve /eks-test-status-upstream")
	return cmd
}

var prowStatusPort string

func prowStatusFunc(cmd *cobra.Command, args []string) {
	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}

	if !strings.HasPrefix(prowStatusPort, ":") {
		fmt.Fprintf(os.Stderr, "invalid prow status port %q\n", prowStatusPort)
		os.Exit(1)
	}

	lg.Info("starting server", zap.String("port", prowStatusPort))

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:    prowStatusPort,
		Handler: status.NewMux(rootCtx, lg),
	}
	errc := make(chan error)
	go func() {
		errc <- srv.ListenAndServe()
	}()

	lg.Info("received signal, shutting down server", zap.String("signal", (<-notifier).String()))
	rootCancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	srv.Shutdown(ctx)
	cancel()
	lg.Info("shut down server", zap.Error(<-errc))

	signal.Stop(notifier)
}

func newProwALB() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alb",
		Short: "Generates ALB Ingress Controller prow job",
		Run:   prowALBFunc,
	}
	cmd.PersistentFlags().StringVar(&prowALBOutputPath, "output-path", "", "file path to output generated Prow job configuration")
	return cmd
}

var prowALBOutputPath string

func prowALBFunc(cmd *cobra.Command, args []string) {
	cfg := eksconfig.NewDefault()
	if err := cfg.UpdateFromEnvs(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to update eksconfig.Config %v\n", err)
		os.Exit(1)
	}
	s, err := alb.CreateProwJobYAML(alb.ConfigProwJobYAML{
		AWSTESTER_EKS_KUBETEST_EMBEDDED_BINARY: fmt.Sprintf("%v", cfg.KubetestEmbeddedBinary),

		AWSTESTER_EKS_WAIT_BEFORE_DOWN: fmt.Sprintf("%v", cfg.WaitBeforeDown),
		AWSTESTER_EKS_DOWN:             fmt.Sprintf("%v", cfg.Down),

		AWSTESTER_EKS_ENABLE_NODE_SSH: fmt.Sprintf("%v", cfg.EnableNodeSSH),

		AWSTESTER_EKS_AWSTESTER_IMAGE: fmt.Sprintf("%s", cfg.AWSTesterImage),

		AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE: fmt.Sprintf("%s", cfg.WorkerNodeInstanceType),
		AWSTESTER_EKS_WORKER_NODE_ASG_MIN:       fmt.Sprintf("%d", cfg.WorkderNodeASGMin),
		AWSTESTER_EKS_WORKER_NODE_ASG_MAX:       fmt.Sprintf("%d", cfg.WorkderNodeASGMax),

		AWSTESTER_EKS_ENABLE_WORKER_NODE_HA: fmt.Sprintf("%v", cfg.EnableWorkerNodeHA),
		AWSTESTER_EKS_ALB_ENABLE:            fmt.Sprintf("%v", cfg.ALBIngressController.Enable),

		AWSTESTER_EKS_LOG_DEBUG:               fmt.Sprintf("%v", cfg.LogDebug),
		AWSTESTER_EKS_LOG_ACCESS:              fmt.Sprintf("%v", cfg.LogAccess),
		AWSTESTER_EKS_UPLOAD_AWS_TESTER_LOGS:  fmt.Sprintf("%v", cfg.UploadAWSTesterLogs),
		AWSTESTER_EKS_ALB_UPLOAD_TESTER_LOGS:  fmt.Sprintf("%v", cfg.ALBIngressController.UploadTesterLogs),
		AWSTESTER_EKS_UPLOAD_WORKER_NODE_LOGS: fmt.Sprintf("%v", cfg.UploadWorkerNodeLogs),

		AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE: cfg.ALBIngressController.ALBIngressControllerImage,

		AWSTESTER_EKS_ALB_TARGET_TYPE: cfg.ALBIngressController.TargetType,
		AWSTESTER_EKS_ALB_TEST_MODE:   cfg.ALBIngressController.TestMode,

		AWSTESTER_EKS_ALB_TEST_SCALABILITY:            fmt.Sprintf("%v", cfg.ALBIngressController.TestScalability),
		AWSTESTER_EKS_ALB_TEST_METRICS:                fmt.Sprintf("%v", cfg.ALBIngressController.TestMetrics),
		AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS:        fmt.Sprintf("%d", cfg.ALBIngressController.TestServerReplicas),
		AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES:          fmt.Sprintf("%d", cfg.ALBIngressController.TestServerRoutes),
		AWSTESTER_EKS_ALB_TEST_CLIENTS:                fmt.Sprintf("%d", cfg.ALBIngressController.TestClients),
		AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS:        fmt.Sprintf("%d", cfg.ALBIngressController.TestClientRequests),
		AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE:          fmt.Sprintf("%d", cfg.ALBIngressController.TestResponseSize),
		AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD: fmt.Sprintf("%d", cfg.ALBIngressController.TestClientErrorThreshold),
		AWSTESTER_EKS_ALB_TEST_EXPECT_QPS:             fmt.Sprintf("%f", cfg.ALBIngressController.TestExpectQPS),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create ALB Prow configuration %v\n", err)
		os.Exit(1)
	}
	if err = ioutil.WriteFile(prowALBOutputPath, []byte(s), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write ALB Prow configuration %v\n", err)
		os.Exit(1)
	}
}
