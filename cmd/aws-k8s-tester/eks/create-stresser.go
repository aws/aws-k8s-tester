package eks

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/stresser"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	stresserKubeConfigPath string

	stresserClients       int
	stresserClientQPS     float32
	stresserClientBurst   int
	stresserClientTimeout time.Duration
	stresserObjectSize    int
	stresserDuration      time.Duration

	stresserNamespaceWrite string
	stresserNamespacesRead []string

	stresserWritesOutputNamePrefix string
	stresserReadsOutputNamePrefix  string

	stresserBlock bool
)

func newCreateStresser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stresser",
		Short: "Creates cluster loader",
		Run:   createStresserFunc,
	}
	cmd.PersistentFlags().StringVar(&stresserKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().IntVar(&stresserClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&stresserClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&stresserClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&stresserClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&stresserObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().DurationVar(&stresserDuration, "duration", 5*time.Minute, "duration to run cluster loader")
	cmd.PersistentFlags().StringVar(&stresserNamespaceWrite, "namespace-write", "default", "namespaces to send writes")
	cmd.PersistentFlags().StringSliceVar(&stresserNamespacesRead, "namespaces-read", []string{"default"}, "namespaces to send reads")
	cmd.PersistentFlags().StringVar(&stresserWritesOutputNamePrefix, "writes-output-name-prefix", "", "writes results output name prefix in /var/log/")
	cmd.PersistentFlags().StringVar(&stresserReadsOutputNamePrefix, "reads-output-name-prefix", "", "reads results output name prefix in /var/log/")
	cmd.PersistentFlags().BoolVar(&stresserBlock, "block", false, "true to block process exit after cluster loader complete")
	return cmd
}

func createStresserFunc(cmd *cobra.Command, args []string) {
	// optional
	if stresserKubeConfigPath != "" && !fileutil.Exist(stresserKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", stresserKubeConfigPath)
		os.Exit(1)
	}
	if err := os.MkdirAll("/var/log", 0700); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create dir %v\n", err)
		os.Exit(1)
	}
	if err := fileutil.IsDirWriteable("/var/log"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write dir %v\n", err)
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: stresserKubeConfigPath,
		Clients:        stresserClients,
		ClientQPS:      stresserClientQPS,
		ClientBurst:    stresserClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	stopc := make(chan struct{})

	loader := stresser.New(stresser.Config{
		Logger:         lg,
		Stopc:          stopc,
		Client:         cli,
		ClientTimeout:  stresserClientTimeout,
		Deadline:       time.Now().Add(stresserDuration),
		NamespaceWrite: stresserNamespaceWrite,
		NamespacesRead: stresserNamespacesRead,
		ObjectSize:     stresserObjectSize,
	})
	loader.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		loader.Stop()
		os.Exit(0)
	case <-time.After(stresserDuration):
	}

	writes, reads, err := loader.GetMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	} else {
		// to randomize results output files
		// when multiple pods are created via deployment
		// we do not want each pod to write to the same file
		// we want to avoid conflicts and run checks for each pod
		// enough for make them unique per worker
		sfx := randutil.String(7)

		writesPath := "/var/log/" + stresserWritesOutputNamePrefix + "-" + sfx + "-writes.json"
		lg.Info("writing writes results output", zap.String("path", writesPath))
		err = ioutil.WriteFile(writesPath, []byte(writes.JSON()), 0600)
		if err != nil {
			lg.Warn("failed to write write results", zap.Error(err))
		}

		readsPath := "/var/log/" + stresserReadsOutputNamePrefix + "-" + sfx + "-reads.json"
		lg.Info("writing reads results output", zap.String("path", readsPath))
		err = ioutil.WriteFile(readsPath, []byte(reads.JSON()), 0600)
		if err != nil {
			lg.Warn("failed to write read results", zap.Error(err))
		}
	}

	close(stopc)
	loader.Stop()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create stresser' success\n")

	if stresserBlock {
		lg.Info("waiting for OS signal")
		select {
		case sig := <-sigs:
			lg.Info("received OS signal", zap.String("signal", sig.String()))
			os.Exit(0)
		}
	}
}
