package eksctl

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/internal"
	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/spf13/pflag"
	"github.com/urfave/sflags/gen/gpflag"
	"sigs.k8s.io/kubetest2/pkg/types"
	"sigs.k8s.io/yaml"
)

// DeployerName is the name of the deployer
const DeployerName = "eksctl"

type deployer struct {
	// generic parts
	commonOptions types.Options
	*UpOptions
	awsConfig      aws.Config
	eksClient      *eks.Client
	KubeconfigPath string `flag:"kubeconfig" desc:"Path to kubeconfig"`
	// ClusterName is the effective cluster name (from flag or RunID)
	clusterName string
}

// NewDeployer implements deployer.New for EKS using eksctl
func NewDeployer(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	// create a deployer object and set fields that are not flag controlled
	awsConfig := awssdk.NewConfig()
	d := &deployer{
		commonOptions: opts,
		awsConfig:     awsConfig,
		eksClient:     eks.NewFromConfig(awsConfig),
	}
	// register flags and return
	return d, bindFlags(d)
}

func (d *deployer) DumpClusterLogs() error {
	return nil
}

func (d *deployer) Kubeconfig() (string, error) {
	if d.KubeconfigPath != "" {
		return d.KubeconfigPath, nil
	}
	return filepath.Join(d.commonOptions.RunDir(), "kubeconfig"), nil
}

func (d *deployer) Version() string {
	return internal.Version
}

// bindFlags is a helper used to create & bind a flagset to the deployer
func bindFlags(d *deployer) *pflag.FlagSet {
	flags, err := gpflag.Parse(d)
	if err != nil {
		slog.Error("unable to bind flags for deployer")
		os.Exit(1)
	}
	flags.AddGoFlagSet(flag.CommandLine)
	return flags
}

// initClusterName sets the effective cluster name with this precedence:
// 1. config file
// 2. --cluster-name flag
// 3. RunID of the kubetest
func (d *deployer) initClusterName() {
	// First priority: config file if provided
	if d.UpOptions.ConfigFile != "" {
		clusterName, err := d.parseClusterNameFromConfig(d.UpOptions.ConfigFile)
		if err == nil {
			d.clusterName = clusterName
			slog.Debug("using cluster name from config file", "clusterName", d.clusterName)
			return
		}
		slog.Warn("failed to extract cluster name from config file", "error", err)
		// Continue with other methods if parsing fails
	}

	if d.UpOptions.ClusterName != "" {
		d.clusterName = d.UpOptions.ClusterName
		slog.Debug("using cluster name from flag", "clusterName", d.clusterName)
	} else {
		d.clusterName = d.commonOptions.RunID()
		slog.Debug("using RunID for cluster name", "clusterName", d.clusterName)
	}
}

// parseClusterNameFromConfig extracts the cluster name from an eksctl config file
func (d *deployer) parseClusterNameFromConfig(configFilePath string) (string, error) {
	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %v", err)
	}

	// Simple YAML parsing to extract the cluster name
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(configData, &configMap); err != nil {
		return "", fmt.Errorf("failed to parse config file YAML: %v", err)
	}

	// Extract metadata section
	metadata, ok := configMap["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("metadata section missing in config file")
	}

	// Extract name field
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return "", fmt.Errorf("cluster name not found in config file metadata")
	}

	return name, nil
}

// assert that deployer implements types.DeployerWithKubeconfig
var _ types.DeployerWithKubeconfig = &deployer{}
