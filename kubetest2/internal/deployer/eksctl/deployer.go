package eksctl

import (
	"flag"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/awssdk"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/types"
)

// DeployerName is the name of the deployer
const DeployerName = "eksctl"

var (
	GitTag string
)

type deployer struct {
	// generic parts
	commonOptions types.Options
	*UpOptions
	eksClient      *eks.EKS
	KubeconfigPath string `flag:"kubeconfig" desc:"Path to kubeconfig"`
}

// NewDeployer implements deployer.New for EKS using eksctl
func NewDeployer(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	// create a deployer object and set fields that are not flag controlled
	d := &deployer{
		commonOptions: opts,
		eksClient:     eks.New(awssdk.NewSession()),
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
	return GitTag
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

// assert that deployer implements types.DeployerWithKubeconfig
var _ types.DeployerWithKubeconfig = &deployer{}
