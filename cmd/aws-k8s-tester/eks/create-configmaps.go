package eks

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	config_maps "github.com/aws/aws-k8s-tester/eks/config-maps"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configmapsKubeConfigPath string

	configmapsClients       int
	configmapsClientQPS     float32
	configmapsClientBurst   int
	configmapsClientTimeout time.Duration
	configmapsObjects       int
	configmapsObjectSize    int

	configmapsNamespace string

	configmapsWritesOutputNamePrefix string

	configmapsBlock bool
)

func newCreateConfigMaps() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configmaps",
		Short: "Creates cluster loader",
		Run:   createConfigMapsFunc,
	}
	cmd.PersistentFlags().StringVar(&configmapsKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().IntVar(&configmapsClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&configmapsClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&configmapsClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&configmapsClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&configmapsObjects, "objects", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().IntVar(&configmapsObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().StringVar(&configmapsNamespace, "namespace", "default", "namespace to send writes")
	cmd.PersistentFlags().StringVar(&configmapsWritesOutputNamePrefix, "writes-output-name-prefix", "", "Write results output name prefix in /var/log/")
	cmd.PersistentFlags().BoolVar(&configmapsBlock, "block", false, "true to block process exit after cluster loader complete")
	return cmd
}

func createConfigMapsFunc(cmd *cobra.Command, args []string) {
	// optional
	if configmapsKubeConfigPath != "" && !fileutil.Exist(configmapsKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", configmapsKubeConfigPath)
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
		KubeConfigPath: configmapsKubeConfigPath,
		Clients:        configmapsClients,
		ClientQPS:      configmapsClientQPS,
		ClientBurst:    configmapsClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	stopc := make(chan struct{})
	donec := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			lg.Info("received OS signal; closing stopc", zap.String("signal", sig.String()))
			close(stopc)
		case <-donec:
		}
	}()

	// to randomize results output files
	// when multiple pods are created via deployment
	// we do not want each pod to write to the same file
	// we want to avoid conflicts and run checks for each pod
	// enough for make them unique per worker
	sfx := randutil.String(7)

	loader := config_maps.New(config_maps.Config{
		Logger:         lg,
		Stopc:          stopc,
		Client:         cli,
		ClientTimeout:  configmapsClientTimeout,
		Namespace:      configmapsNamespace,
		Objects:        configmapsObjects,
		ObjectSize:     configmapsObjectSize,
		WritesJSONPath: "/var/log/" + configmapsWritesOutputNamePrefix + "-" + sfx + "-writes.json",
	})
	loader.Start()
	loader.Stop()
	close(donec)

	writes, err := loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	} else {
		writesPath := "/var/log/" + configmapsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json"
		lg.Info("writing writes results output", zap.String("path", writesPath))
		err = ioutil.WriteFile(writesPath, []byte(writes.JSON()), 0600)
		if err != nil {
			lg.Warn("failed to write write results", zap.Error(err))
		}
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create configmaps' success\n")

	if configmapsBlock {
		lg.Info("waiting for OS signal")
		select {
		case sig := <-sigs:
			lg.Info("received OS signal", zap.String("signal", sig.String()))
			os.Exit(0)
		}
	}
}
