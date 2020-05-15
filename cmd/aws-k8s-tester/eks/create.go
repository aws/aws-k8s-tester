package eks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks"
	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	hollow_nodes "github.com/aws/aws-k8s-tester/eks/hollow-nodes"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newCreate() *cobra.Command {
	ac := &cobra.Command{
		Use:   "create <subcommand>",
		Short: "Create commands",
	}
	ac.AddCommand(
		newCreateConfig(),
		newCreateCluster(),
		newCreateHollowNodes(),
		newCreateClusterLoader(),
	)
	return ac
}

func newCreateConfig() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Writes an aws-k8s-tester eks configuration with default values",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createConfigFunc,
	}
}

func createConfigFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}
	cfg := eksconfig.NewDefault()
	cfg.ConfigPath = path

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables...\n")
	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester eks create config' fail %v\n", err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	println()
	fmt.Println(string(txt))
	println()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create config' success\n")
}

func newCreateCluster() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Create an eks cluster",
		Long:  "Configuration values are overwritten by environment variables.",
		Run:   createClusterFunc,
	}
	return cmd
}

func createClusterFunc(cmd *cobra.Command, args []string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "'--path' flag is not specified")
		os.Exit(1)
	}

	var cfg *eksconfig.Config
	var err error
	if fileutil.Exist(path) {
		cfg, err = eksconfig.Load(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
		if err = cfg.ValidateAndSetDefaults(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "cannot find configuration %q; writing...\n", path)
		cfg = eksconfig.NewDefault()
		cfg.ConfigPath = path
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("overwriting config file from environment variables...\n")
	err = cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	txt, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration %q (%v)\n", path, err)
		os.Exit(1)
	}
	println()
	fmt.Println(string(txt))
	println()

	if enablePrompt {
		prompt := promptui.Select{
			Label: "Ready to create EKS resources, should we continue?",
			Items: []string{
				"No, cancel it!",
				"Yes, let's create!",
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("returning 'create' [index %d, answer %q]\n", idx, answer)
			return
		}
	}

	time.Sleep(5 * time.Second)

	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	if err = tester.Up(); err != nil {
		fmt.Printf("\n*********************************\n")
		fmt.Printf("'aws-k8s-tester eks create cluster' fail %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create cluster' success\n")
}

var (
	hollowNodesPrefix         string
	hollowNodesKubeConfigPath string
	hollowNodesNodes          int

	hollowNodesClients     int
	hollowNodesClientQPS   float32
	hollowNodesClientBurst int
)

func newCreateHollowNodes() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hollow-nodes",
		Short: "Creates hollow nodes",
		Run:   createHollowNodesFunc,
	}
	cmd.PersistentFlags().StringVar(&hollowNodesPrefix, "prefix", randutil.String(5), "Prefix to label hollow node groups")
	cmd.PersistentFlags().StringVar(&hollowNodesKubeConfigPath, "kubeconfig", "", "kubeconfig path")
	cmd.PersistentFlags().IntVar(&hollowNodesNodes, "nodes", 10, "Number of hollow nodes to create")
	cmd.PersistentFlags().IntVar(&hollowNodesClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&hollowNodesClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&hollowNodesClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	return cmd
}

func createHollowNodesFunc(cmd *cobra.Command, args []string) {
	// optional
	if hollowNodesKubeConfigPath != "" && !fileutil.Exist(hollowNodesKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", hollowNodesKubeConfigPath)
		os.Exit(1)
	}
	if len(hollowNodesPrefix) > 40 {
		fmt.Fprintf(os.Stderr, "invalid node label prefix %q (%d characters, label value can not be more than 63 characters)\n", hollowNodesPrefix, len(hollowNodesPrefix))
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: hollowNodesKubeConfigPath,
		Clients:        hollowNodesClients,
		ClientQPS:      hollowNodesClientQPS,
		ClientBurst:    hollowNodesClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	sfx := randutil.String(5)

	stopc := make(chan struct{})
	ng, err := hollow_nodes.CreateNodeGroup(hollow_nodes.NodeGroupConfig{
		Logger: lg,
		Client: cli,
		Stopc:  stopc,
		Nodes:  hollowNodesNodes,
		NodeLabels: map[string]string{
			"AMIType": hollowNodesPrefix + "-ami-type-" + sfx,
			"NGType":  hollowNodesPrefix + "-ng-type-" + sfx,
			"NGName":  hollowNodesPrefix + "-ng-name-" + sfx,
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create hollow nodes group %v\n", err)
		os.Exit(1)
	}

	ng.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		ng.Stop()
		os.Exit(0)
	case <-time.After(10 * time.Second):
	}

	time.Sleep(10 * time.Second)
	readies, created, err := ng.CheckNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to check hollow nodes group %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nready node names: %q\n", readies)
	fmt.Printf("created node names: %q\n\n", created)

	println()
	fmt.Println("waiting for os.Signal...")
	println()

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		close(stopc)
		ng.Stop()
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create hollow-nodes' success\n")
}

var (
	clusterLoaderPrefix         string
	clusterLoaderKubeConfigPath string

	clusterLoaderClients     int
	clusterLoaderClientQPS   float32
	clusterLoaderClientBurst int
	clusterLoaderNamespaces  []string

	clusterLoaderDuration         time.Duration
	clusterLoaderOutputPathPrefix string
	clusterLoaderBlock            bool
)

func newCreateClusterLoader() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster-loader",
		Short: "Creates cluster loader",
		Run:   createClusterLoaderFunc,
	}
	cmd.PersistentFlags().StringVar(&clusterLoaderKubeConfigPath, "kubeconfig", "", "kubeconfig path")
	cmd.PersistentFlags().IntVar(&clusterLoaderClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&clusterLoaderClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&clusterLoaderClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	cmd.PersistentFlags().StringSliceVar(&clusterLoaderNamespaces, "namespaces", []string{"default"}, "namespaces to send reads")
	cmd.PersistentFlags().DurationVar(&clusterLoaderDuration, "duration", 5*time.Minute, "duration to run cluster loader")
	cmd.PersistentFlags().StringVar(&clusterLoaderOutputPathPrefix, "output-path-prefix", "/var/log/cluster-loader-remote-", "Results output path")
	cmd.PersistentFlags().BoolVar(&clusterLoaderBlock, "block", false, "true to block process exit after cluster loader complete")
	return cmd
}

func createClusterLoaderFunc(cmd *cobra.Command, args []string) {
	// optional
	if clusterLoaderKubeConfigPath != "" && !fileutil.Exist(clusterLoaderKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", clusterLoaderKubeConfigPath)
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	cli, err := k8s_client.NewEKS(&k8s_client.EKSConfig{
		Logger:         lg,
		KubeConfigPath: clusterLoaderKubeConfigPath,
		Clients:        clusterLoaderClients,
		ClientQPS:      clusterLoaderClientQPS,
		ClientBurst:    clusterLoaderClientBurst,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create client %v\n", err)
		os.Exit(1)
	}

	stopc := make(chan struct{})

	loader := cluster_loader.New(cluster_loader.Config{
		Logger:     lg,
		Client:     cli,
		Groups:     clusterLoaderClients,
		Stopc:      stopc,
		Deadline:   time.Now().Add(clusterLoaderDuration),
		Timeout:    10 * time.Second,
		Namespaces: clusterLoaderNamespaces,
	})
	loader.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigs:
		lg.Info("received OS signal", zap.String("signal", sig.String()))
		os.Exit(0)
	case <-time.After(clusterLoaderDuration):
	}

	outputPath := clusterLoaderOutputPathPrefix + "-" + randutil.String(5) + ".json"
	if err = os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create dir %v\n", err)
		os.Exit(1)
	}
	if err = fileutil.IsDirWriteable(filepath.Dir(outputPath)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write dir %v\n", err)
		os.Exit(1)
	}

	success, failure, hs, err := loader.GetMetrics()
	if err != nil {
		lg.Warn("failed to get metrics", zap.Error(err))
	} else {
		rs := eksconfig.RequestsSummary{
			SuccessTotal:     success,
			FailureTotal:     failure,
			LatencyHistogram: hs,
		}
		b, err := json.Marshal(rs)
		if err != nil {
			lg.Warn("failed to marshal metrics", zap.Error(err))
		} else {
			lg.Info("writing results output", zap.String("output", outputPath))
			err = ioutil.WriteFile(outputPath, b, 0600)
			if err != nil {
				lg.Warn("failed to write file", zap.Error(err))
			}
		}
	}

	close(stopc)
	loader.Stop()

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create cluster-loader' success\n")

	if clusterLoaderBlock {
		lg.Info("waiting for OS signal")
		select {
		case sig := <-sigs:
			lg.Info("received OS signal", zap.String("signal", sig.String()))
			os.Exit(0)
		}
	}
}
