package eks

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/eks"
	hollow_nodes "github.com/aws/aws-k8s-tester/eks/hollow-nodes"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
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
	fmt.Printf("'aws-k8s-tester eks create config' successs\n")
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
	fmt.Printf("'aws-k8s-tester eks create cluster' successs\n")
}

var (
	hollowNodesPrefix             string
	hollowNodesKubectlPath        string
	hollowNodesKubectlDownloadURL string
	hollowNodesKubeConfigPath     string
	hollowNodesNodes              int

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
	cmd.PersistentFlags().StringVar(&hollowNodesPrefix, "prefix", string(randBytes(5)), "Prefix to label hollow node groups")
	cmd.PersistentFlags().StringVar(&hollowNodesKubectlPath, "kubectl", "", "kubectl path")
	cmd.PersistentFlags().StringVar(&hollowNodesKubectlDownloadURL, "kubectl-download-url", "https://storage.googleapis.com/kubernetes-release/release/v1.16.9/bin/linux/amd64/kubectl", "kubectl download URL")
	cmd.PersistentFlags().StringVar(&hollowNodesKubeConfigPath, "kubeconfig", "", "kubeconfig path")
	cmd.PersistentFlags().IntVar(&hollowNodesNodes, "nodes", 10, "Number of hollow nodes to create")
	cmd.PersistentFlags().IntVar(&hollowNodesClients, "clients", eksconfig.DefaultClients, "Number of clients to create")
	cmd.PersistentFlags().Float32Var(&hollowNodesClientQPS, "client-qps", eksconfig.DefaultClientQPS, "kubelet client setup for QPS")
	cmd.PersistentFlags().IntVar(&hollowNodesClientBurst, "client-burst", eksconfig.DefaultClientBurst, "kubelet client setup for burst")
	return cmd
}

func createHollowNodesFunc(cmd *cobra.Command, args []string) {
	if !fileutil.Exist(hollowNodesKubeConfigPath) {
		fmt.Fprintf(os.Stderr, "kubeconfig not found %q\n", hollowNodesKubeConfigPath)
		os.Exit(1)
	}

	lg, err := logutil.GetDefaultZapLogger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger %v\n", err)
		os.Exit(1)
	}

	lg.Info("mkdir", zap.String("kubectl-path-dir", filepath.Dir(hollowNodesKubectlPath)))
	if err := os.MkdirAll(filepath.Dir(hollowNodesKubectlPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "could not create %q (%v)", filepath.Dir(hollowNodesKubectlPath), err)
		os.Exit(1)
	}
	if !fileutil.Exist(hollowNodesKubectlPath) {
		hollowNodesKubectlPath, _ = filepath.Abs(hollowNodesKubectlPath)
		lg.Info("downloading kubectl", zap.String("kubectl-path", hollowNodesKubectlPath))
		if err := httputil.Download(lg, os.Stderr, hollowNodesKubectlDownloadURL, hollowNodesKubectlPath); err != nil {
			fmt.Fprintf(os.Stderr, "failed to download kubectl %v\n", err)
			os.Exit(1)
		}
	} else {
		lg.Info("skipping kubectl download; already exist", zap.String("kubectl-path", hollowNodesKubectlPath))
	}
	if err := fileutil.EnsureExecutable(hollowNodesKubectlPath); err != nil {
		// file may be already executable while the process does not own the file/directory
		// ref. https://github.com/aws/aws-k8s-tester/issues/66
		lg.Warn("failed to ensure executable", zap.Error(err))
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

	sfx := string(randBytes(5))

	stopc := make(chan struct{})
	ng, err := hollow_nodes.CreateNodeGroup(hollow_nodes.NodeGroupConfig{
		Logger: lg,
		Client: cli,
		Stopc:  stopc,
		Nodes:  hollowNodesNodes,
		NodeLabels: map[string]string{
			"NGType":  hollowNodesPrefix + "-ng-type-" + sfx,
			"AMIType": hollowNodesPrefix + "-ami-type-" + sfx,
			"NGName":  hollowNodesPrefix + "-ng-name-" + sfx,
		},
		KubectlPath:    hollowNodesKubectlPath,
		KubeConfigPath: hollowNodesKubeConfigPath,
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
		fmt.Printf("signal received %v\n", sig)
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
		fmt.Printf("signal received %v\n", sig)
		close(stopc)
		ng.Stop()
		os.Exit(0)
	}

	fmt.Printf("\n*********************************\n")
	fmt.Printf("'aws-k8s-tester eks create hollow-nodes' successs\n")
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return b
}
