package eksapi

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/awssdk"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/util"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/types"
)

// DeployerName is the name of the deployer
const DeployerName = "eksapi"

const ResourcePrefix = "kubetest2-" + DeployerName

// assert that deployer implements types.DeployerWithKubeconfig
var _ types.DeployerWithKubeconfig = &deployer{}

type deployer struct {
	commonOptions types.Options
	deployerOptions

	awsConfig aws.Config

	_awsClients *awsClients

	infra   *infra
	cluster *cluster

	resourceID string
}

type deployerOptions struct {
	Region                      string        `flag:"region" desc:"AWS region for EKS cluster"`
	KubernetesVersion           string        `flag:"kubernetes-version" desc:"cluster Kubernetes version"`
	Nodes                       int           `flag:"nodes" desc:"number of nodes to launch in cluster"`
	AMI                         string        `flag:"ami" desc:"Node AMI"`
	InstanceTypes               []string      `flag:"instance-types" desc:"Node instance types"`
	ClusterRoleServicePrincipal string        `flag:"cluster-role-service-principal" desc:"Additional service principal that can assume the cluster role"`
	KubeconfigPath              string        `flag:"kubeconfig" desc:"Path to kubeconfig"`
	EKSEndpointURL              string        `flag:"endpoint-url" desc:"Endpoint URL for the EKS API"`
	UpClusterHeaders            []string      `flag:"up-cluster-header" desc:"Additional header to add to eks:CreateCluster requests. Specified in the same format as curl's -H flag."`
	IPFamily                    string        `flag:"ip-family" desc:"IP family for the cluster (ipv4 or ipv6)"`
	UnmanagedNodes              bool          `flag:"unmanaged-nodes" desc:"Use an AutoScalingGroup instead of an EKS-managed nodegroup."`
	NodeReadyTimeout            time.Duration `flag:"node-ready-timeout" desc:"Time to wait for all nodes to become ready"`
	GenerateSSHKey              bool          `flag:"generate-ssh-key" desc:"Generate an SSH key to use for tests. The generated key should not be used in production, as it will not have a passphrase."`
}

// NewDeployer implements deployer.New for EKS using the EKS (and other AWS) API(s) directly (no cloudformation)
func NewDeployer(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	// create a deployer object and set fields that are not flag controlled
	awsConfig := awssdk.NewConfig()
	d := &deployer{
		commonOptions: opts,
		awsConfig:     awsConfig,
		resourceID:    ResourcePrefix + "-" + opts.RunID(),
	}
	// register flags and return
	return d, bindFlags(d)
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

func (d *deployer) Version() string {
	return internal.Version
}

// Build is a no-op
func (d *deployer) Build() error {
	return nil
}

// DumpClusterLogs is a no-op
func (d *deployer) DumpClusterLogs() error {
	return nil
}

func (d *deployer) Kubeconfig() (string, error) {
	if d.KubeconfigPath == "" {
		kubeconfigPath := filepath.Join(d.commonOptions.RunDir(), "kubeconfig")
		err := writeKubeconfig(d.cluster, kubeconfigPath)
		if err != nil {
			klog.Warningf("failed to write kubeconfig: %v", err)
			return "", err
		}
		d.KubeconfigPath = kubeconfigPath
	}
	return d.KubeconfigPath, nil
}

func (d *deployer) awsClients() *awsClients {
	if d._awsClients == nil {
		d._awsClients = newAWSClients(d.awsConfig, d.EKSEndpointURL)
	}
	return d._awsClients
}

func (d *deployer) Up() error {
	if err := d.verifyUpFlags(); err != nil {
		return fmt.Errorf("up flags are invalid: %v", err)
	}
	if d.GenerateSSHKey {
		if err := generateSSHKey(); err != nil {
			return err
		}
	}
	if infra, err := createInfrastructureStack(d.awsClients(), &d.deployerOptions, d.resourceID); err != nil {
		return err
	} else {
		d.infra = infra
	}
	cluster, err := createCluster(d.awsClients(), d.infra, &d.deployerOptions, d.resourceID)
	if err != nil {
		return err
	}
	d.cluster = cluster
	kubeconfig, err := d.Kubeconfig()
	if err != nil {
		return err
	}
	k8sClient, err := newKubernetesClient(kubeconfig)
	if err != nil {
		return err
	}
	if d.UnmanagedNodes {
		if err := createAWSAuthConfigMap(k8sClient, d.infra.nodeRole); err != nil {
			return err
		}
	}
	if err := createNodegroup(d.awsClients(), d.infra, d.cluster, &d.deployerOptions, d.resourceID); err != nil {
		return err
	}
	if err := waitForReadyNodes(k8sClient, d.Nodes, d.NodeReadyTimeout); err != nil {
		return err
	}
	return nil
}

func (d *deployer) verifyUpFlags() error {
	if d.KubernetesVersion == "" {
		klog.Infof("--kubernetes-version is empty, attempting to detect it...")
		detectedVersion, err := detectKubernetesVersion()
		if err != nil {
			return fmt.Errorf("unable to detect --kubernetes-version, flag cannot be empty")
		}
		klog.Infof("detected --kubernetes-version=%s", detectedVersion)
		d.KubernetesVersion = detectedVersion
	}
	if d.Nodes < 0 {
		return fmt.Errorf("number of nodes must be greater than zero")
	}
	if d.Nodes == 0 {
		d.Nodes = 3
		klog.V(2).Infof("Using default number of nodes: %d", d.Nodes)
	}
	if d.IPFamily == "" {
		d.IPFamily = string(ekstypes.IpFamilyIpv4)
		klog.V(2).Infof("Using default IP family: %s", d.IPFamily)
	}
	if len(d.InstanceTypes) == 0 && d.UnmanagedNodes {
		d.InstanceTypes = []string{
			"m7i.large",
			"m6i.large",
			"m6a.large",
			"m5.large",
			"m5a.large",
			"m4.large",
		}
		klog.V(2).Infof("Using default instance types: %v", d.InstanceTypes)
	}
	if d.NodeReadyTimeout == 0 {
		d.NodeReadyTimeout = time.Minute * 5
	}
	return nil
}

func detectKubernetesVersion() (string, error) {
	detectedVersion, err := util.DetectKubernetesVersion()
	if err != nil {
		return "", err
	}
	minorVersion, err := util.ParseMinorVersion(detectedVersion)
	if err != nil {
		return "", err
	}
	return minorVersion, nil
}

func (d *deployer) IsUp() (up bool, err error) {
	result, err := d.awsClients().EKS().DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
		Name: aws.String(d.resourceID),
	})
	if err != nil {
		return false, err
	}
	switch result.Cluster.Status {
	case ekstypes.ClusterStatusActive:
		return true, nil
	case ekstypes.ClusterStatusCreating:
		return false, nil
	default:
		return false, fmt.Errorf("cluster status is: %v", result.Cluster.Status)
	}
}

func (d *deployer) Down() error {
	return deleteResources(d.awsClients(), d.resourceID)
}

func deleteResources(clients *awsClients, resourceID string) error {
	if err := deleteNodegroup(clients, resourceID); err != nil {
		return err
	}
	if err := deleteCluster(clients, resourceID); err != nil {
		return err
	}
	if err := deleteLeakedENIs(clients, resourceID); err != nil {
		return err
	}
	return deleteInfrastructureStack(clients, resourceID)
}
