package eks

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks/secrets"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	secretsKubeConfigPath string

	secretsClients       int
	secretsClientQPS     float32
	secretsClientBurst   int
	secretsClientTimeout time.Duration
	secretsObjects       int
	secretsObjectSize    int

	secretsNamespace  string
	secretsNamePrefix string

	secretsWritesOutputNamePrefix string
	secretsReadsOutputNamePrefix  string

	secretsBlock bool
)

func newCreateSecrets() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Creates cluster loader",
		Run:   createSecretsFunc,
	}
	cmd.PersistentFlags().StringVar(&secretsKubeConfigPath, "kubeconfig", "", "kubeconfig path (optional, should be run in-cluster, useful for local testing)")
	cmd.PersistentFlags().IntVar(&secretsClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&secretsClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&secretsClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().DurationVar(&secretsClientTimeout, "client-timeout", eksconfig.DefaultClientTimeout, "kubelet client timeout")
	cmd.PersistentFlags().IntVar(&secretsObjects, "objects", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().IntVar(&secretsObjectSize, "object-size", 0, "Size of object per write (0 to disable writes)")
	cmd.PersistentFlags().StringVar(&secretsNamespace, "namespace", "default", "namespace to send writes")
	cmd.PersistentFlags().StringVar(&secretsNamePrefix, "name-prefix", "", "prefix of Secret name")
	cmd.PersistentFlags().StringVar(&secretsWritesOutputNamePrefix, "writes-output-name-prefix", "", "writes results output name prefix in /var/log/")
	cmd.PersistentFlags().StringVar(&secretsReadsOutputNamePrefix, "reads-output-name-prefix", "", "reads results output name prefix in /var/log/")
	cmd.PersistentFlags().BoolVar(&secretsBlock, "block", false, "true to block process exit after cluster loader complete")
	return cmd
}

func createSecretsFunc(cmd *cobra.Command, args []string) {
	// optional
	if secretsKubeConfigPath != "" && !fileutil.Exist(secretsKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", secretsKubeConfigPath)
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
		KubeConfigPath: secretsKubeConfigPath,
		Clients:        secretsClients,
		ClientQPS:      secretsClientQPS,
		ClientBurst:    secretsClientBurst,
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

	loader := secrets.New(secrets.Config{
		Logger:         lg,
		Stopc:          stopc,
		Client:         cli,
		ClientTimeout:  secretsClientTimeout,
		Namespace:      secretsNamespace,
		NamePrefix:     secretsNamePrefix,
		Objects:        secretsObjects,
		ObjectSize:     secretsObjectSize,
		WritesJSONPath: "/var/log/" + secretsWritesOutputNamePrefix + "-" + sfx + "-writes.json",
		ReadsJSONPath:  "/var/log/" + secretsReadsOutputNamePrefix + "-" + sfx + "-reads.json",
	})
	loader.Start()
	loader.Stop()
	close(donec)

	writes, reads, err := loader.CollectMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	} else {
		writesPath := "/var/log/" + secretsWritesOutputNamePrefix + "-" + sfx + "-writes-summary.json"
		lg.Info("writing writes results output", zap.String("path", writesPath))
		err = ioutil.WriteFile(writesPath, []byte(writes.JSON()), 0600)
		if err != nil {
			lg.Warn("failed to write write results", zap.Error(err))
		}

		readsPath := "/var/log/" + secretsReadsOutputNamePrefix + "-" + sfx + "-reads-summary.json"
		lg.Info("writing reads results output", zap.String("path", readsPath))
		err = ioutil.WriteFile(readsPath, []byte(reads.JSON()), 0600)
		if err != nil {
			lg.Warn("failed to write write results", zap.Error(err))
		}
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create secrets' success\n")

	if secretsBlock {
		lg.Info("waiting for OS signal")
		select {
		case sig := <-sigs:
			lg.Info("received OS signal", zap.String("signal", sig.String()))
			os.Exit(0)
		}
	}
}
