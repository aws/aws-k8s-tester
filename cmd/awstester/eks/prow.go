package eks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
