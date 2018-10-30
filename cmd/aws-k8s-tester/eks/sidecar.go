package eks

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/internal/sidecar"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

/*
curl -X POST "http://localhost:32050/sidecar" \
  -H "accept: application/json" \
  --data '{"target-url":"https://httpbin.org/get","method":"GET"}'
*/

func newSidecar() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sidecar",
		Short: "Start sidecar server",
		Run:   sidecarFunc,
	}
	cmd.PersistentFlags().StringVar(&sidecarPort, "port", ":32050", "specify the sidecar server port")
	return cmd
}

var sidecarPort string

func sidecarFunc(cmd *cobra.Command, args []string) {
	lg, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	lg.Info("starting 'sidecar'", zap.String("port", sidecarPort))

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	var mux *http.ServeMux
	mux, err = sidecar.NewMux(rootCtx, lg, "/sidecar")
	if err != nil {
		panic(err)
	}
	srv := &http.Server{
		Addr:    sidecarPort,
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
