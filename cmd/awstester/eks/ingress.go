package eks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/internal/eks/alb/ingress/client"
	"github.com/aws/awstester/internal/eks/alb/ingress/server"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newIngress() *cobra.Command {
	ac := &cobra.Command{
		Use:   "ingress <subcommand>",
		Short: "Ingress commands",
	}
	ac.AddCommand(
		newIngressServer(),
		newIngressClient(),
	)
	return ac
}

func newIngressServer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run ingress test server",
		Run:   ingressServerFunc,
	}
	cmd.PersistentFlags().StringVar(&ingressServerPort, "port", ":32030", "specify the ingress test server port")
	cmd.PersistentFlags().IntVar(&ingressServerRoutes, "routes", 3, "specify the number of routes (e.g. /ingress-test-00001, /ingress-test-00002, and so on)")
	cmd.PersistentFlags().IntVar(&ingressServerResponseSize, "response-size", 40*1024, "specify the server response size")
	return cmd
}

var (
	ingressServerPort         string
	ingressServerRoutes       int
	ingressServerResponseSize int
)

func ingressServerFunc(cmd *cobra.Command, args []string) {
	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	lg.Info(
		"starting ingress server",
		zap.String("port", ingressServerPort),
		zap.Int("routes", ingressServerRoutes),
		zap.String("response-size", humanize.Bytes(uint64(ingressServerResponseSize))),
	)

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	var mux *http.ServeMux
	mux, err = server.NewMux(rootCtx, lg, ingressServerRoutes, ingressServerResponseSize)
	if err != nil {
		panic(err)
	}
	srv := &http.Server{
		Addr:    ingressServerPort,
		Handler: mux,
	}
	errc := make(chan error)
	go func() {
		lg.Info("started serving")
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

func newIngressClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Runs ingress test client",
		Run:   ingressClientFunc,
	}
	cmd.PersistentFlags().StringVar(&ingressClientEp, "endpoint", "", "ingress-test server endpoint")
	cmd.PersistentFlags().IntVar(&ingressClientRoutes, "routes", 10, "total number of routes")
	cmd.PersistentFlags().IntVar(&ingressClientClients, "clients", 100, "total number of concurrent clients")
	cmd.PersistentFlags().IntVar(&ingressClientRequests, "requests", 5000, "total number of requests")
	cmd.PersistentFlags().StringVar(&ingressClientResultPath, "result-path", "", "file path to output results in encoded 'eksconfig.Config' YAML")
	return cmd
}

var (
	ingressClientEp         string
	ingressClientRoutes     int
	ingressClientClients    int
	ingressClientRequests   int
	ingressClientResultPath string
)

func ingressClientFunc(cmd *cobra.Command, args []string) {
	if ingressClientEp == "" {
		fmt.Fprintf(os.Stderr, "invalid ingress test endpoint %q", ingressClientEp)
		os.Exit(1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	// send loads from client to server
	cli, err := client.New(lg, ingressClientEp, ingressClientRoutes, ingressClientClients, ingressClientRequests)
	if err != nil {
		lg.Fatal("failed to create client", zap.Error(err))
	}

	lg.Info("starting ingress client")
	rs := cli.Run()
	lg.Info("finished ingress client")

	fmt.Println(rs)

	if ingressClientResultPath != "" {
		cfg := &eksconfig.Config{
			ConfigPath:   ingressClientResultPath,
			ClusterState: &eksconfig.ClusterState{},
			ALBIngressController: &eksconfig.ALBIngressController{
				TestResultQPS:      rs.QPS,
				TestResultFailures: rs.Failure,
			},
		}
		cfg.Sync()
		lg.Info("saved test results using 'eksconfig.Config' struct", zap.String("result-path", ingressClientResultPath))
	}

	if len(rs.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "expected not error, got %v", rs.Errors)
		os.Exit(1)
	}
	if rs.Failure > 0 {
		fmt.Fprintf(os.Stderr, "expected no failure, got %v", rs.Failure)
		os.Exit(1)
	}
}
