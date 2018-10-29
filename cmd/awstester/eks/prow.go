package eks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aws/awstester/internal/prow/status"
	"github.com/aws/awstester/pkg/fileutil"

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

func newProwStatus() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Serve Kubernetes upstream test status",
	}
	cmd.AddCommand(
		newProwStatusServe(),
		newProwStatusGet(),
	)
	return cmd
}

/*
http://localhost:32010/eks-test-status-upstream
*/
func newProwStatusServe() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve Kubernetes upstream test status",
		Run:   prowStatusServeFunc,
	}
	cmd.PersistentFlags().StringVar(&prowStatusServePort, "port", ":32010", "port to serve /eks-test-status-upstream")
	return cmd
}

var prowStatusServePort string

func prowStatusServeFunc(cmd *cobra.Command, args []string) {
	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}

	if !strings.HasPrefix(prowStatusServePort, ":") {
		fmt.Fprintf(os.Stderr, "invalid prow status port %q\n", prowStatusServePort)
		os.Exit(1)
	}

	lg.Info("starting server", zap.String("port", prowStatusServePort))

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)

	rootCtx, rootCancel := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:    prowStatusServePort,
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

func newProwStatusGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Output Kubernetes upstream test status",
		Run:   prowStatusGetFunc,
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get working directory %v\n", err)
		os.Exit(1)
	}
	cmd.PersistentFlags().StringVar(&prowStatusGetDataDir, "data-dir", filepath.Join(wd, "data"), "target directory to output test status")
	return cmd
}

var prowStatusGetDataDir string

/*
go install -v ./cmd/awstester

awstester \
  eks \
  prow \
  status-get
*/
func prowStatusGetFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(prowStatusGetDataDir) {
		if err := os.MkdirAll(prowStatusGetDataDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "failed to mkdir %v\n", err)
			os.Exit(1)
		}
	}
	now := time.Now().UTC()
	p := filepath.Join(prowStatusGetDataDir, fmt.Sprintf("prow-status-%d%02d%02d", now.Year(), now.Month(), now.Day()))

	mux := status.NewMux(context.Background(), zap.NewExample())
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + status.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get %q (%v)\n", status.Path, err)
		os.Exit(1)
	}
	var d []byte
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %q (%v)\n", status.Path, err)
		os.Exit(1)
	}
	resp.Body.Close()
	if err = ioutil.WriteFile(p+".html", d, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %q (%v)\n", p+".html", err)
		os.Exit(1)
	}

	resp, err = http.Get(ts.URL + status.PathSummary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get %q (%v)\n", status.PathSummary, err)
		os.Exit(1)
	}
	d, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %q (%v)\n", status.PathSummary, err)
		os.Exit(1)
	}
	resp.Body.Close()
	if err = ioutil.WriteFile(p+".txt", d, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %q (%v)\n", p+".txt", err)
		os.Exit(1)
	}

	fmt.Printf("saved to %q and %q\n", p+".html", p+".txt")
}
