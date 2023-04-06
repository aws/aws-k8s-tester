package deployer

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/artifacts"
	"sigs.k8s.io/kubetest2/pkg/types"

	"github.com/aws/aws-k8s-tester/eks"
	"github.com/aws/aws-k8s-tester/eksconfig"
)

// Name is the name of the deployer
const Name = "eks"

// assert that New implements types.NewDeployer
var _ types.NewDeployer = New

// New implements deployer.New for eks
func New(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	tester, err := newEKSTester()
	if err != nil {
		fmt.Printf("Failed to create EKS Tester based on Configuration: %v \n", err)
		os.Exit(1)
	}

	d := &deployer{
		tester:        tester,
		commonOptions: opts,
		logsDir:       filepath.Join(artifacts.BaseDir(), "logs"),
	}
	// register flags and return
	return d, bindFlags(d)
}

type deployer struct {
	commonOptions types.Options
	tester        *eks.Tester
	logsDir       string
}

func (d *deployer) Up() error {
	return d.tester.Up()
}

func (d *deployer) Down() error {
	return d.tester.Down()
}

func (d *deployer) IsUp() (bool, error) {
	return d.tester.IsUp()
}

func (d *deployer) DumpClusterLogs() error {
	return d.tester.DownloadClusterLogs(d.logsDir, "")
}

func (d *deployer) Build() error {
	return d.tester.Build()
}

// helper used to create & bind a flagset to the deployer
func bindFlags(d *deployer) *pflag.FlagSet {
	flags, err := gpflag.Parse(d)
	if err != nil {
		klog.Fatalf("unable to generate flags from deployer")
		return nil
	}

	klog.InitFlags(nil)
	flags.AddGoFlagSet(flag.CommandLine)

	return flags
}

func newEKSTester() (*eks.Tester, error) {
	cfg := eksconfig.NewDefault()
	path := filepath.Join(os.TempDir(), cfg.Name+".yaml")
	cfg.ConfigPath = path

	err := cfg.UpdateFromEnvs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration from environment variables: %v\n", err)
		os.Exit(1)
	}

	if err = cfg.ValidateAndSetDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to validate configuration %q (%v)\n", path, err)
		os.Exit(1)
	}

	if cfg.IsEnabledAddOnNodeGroups() {
		body, err := json.MarshalIndent(cfg.AddOnNodeGroups, "", "    ")
		if err != nil {
			panic(err)
		}
		fmt.Printf("AddOnNodeGroups:\n\n%s\n\n\n", string(body))
	}
	if cfg.IsEnabledAddOnManagedNodeGroups() {
		body, err := json.MarshalIndent(cfg.AddOnManagedNodeGroups, "", "    ")
		if err != nil {
			panic(err)
		}
		fmt.Printf("AddOnManagedNodeGroups:\n\n%s\n\n\n", string(body))
	}

	time.Sleep(5 * time.Second)
	tester, err := eks.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create EKS deployer %v\n", err)
		os.Exit(1)
	}

	return tester, nil
}
