package eksctl

import (
	"flag"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/internal"
	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/types"
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
		klog.Fatalf("unable to bind flags for deployer")
		return nil
	}
	klog.InitFlags(nil)
	flags.AddGoFlagSet(flag.CommandLine)
	return flags
}

// initClusterName sets the effective cluster name from flag or RunID
func (d *deployer) initClusterName() {
	if d.UpOptions.ClusterName != "" {
		d.clusterName = d.UpOptions.ClusterName
	} else {
		d.clusterName = d.commonOptions.RunID()
		klog.V(2).Infof("Using RunID for cluster name: %s", d.clusterName)
	}
}

// assert that deployer implements types.DeployerWithKubeconfig
var _ types.DeployerWithKubeconfig = &deployer{}
