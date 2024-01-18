package eksapi

import (
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/awssdk"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/metrics"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/util"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/types"
)

// DeployerName is the name of the deployer
const DeployerName = "eksapi"

const ResourcePrefix = "kubetest2-" + DeployerName

var SupportedOsDistro = []string{"al2", "al2023"}

var DeployerMetricNamespace = path.Join("kubetest2", DeployerName)

var (
	totalRuntimeSeconds = &metrics.MetricSpec{
		Namespace: DeployerMetricNamespace,
		Metric:    "TotalRuntimeSeconds",
		Unit:      cloudwatchtypes.StandardUnitSeconds,
	}
)

// assert that deployer implements optional interfaces
var _ types.DeployerWithKubeconfig = &deployer{}
var _ types.DeployerWithInit = &deployer{}
var _ types.DeployerWithFinish = &deployer{}

type deployer struct {
	commonOptions types.Options
	deployerOptions

	metrics          metrics.MetricRegistry
	infraManager     *InfrastructureManager
	clusterManager   *ClusterManager
	nodegroupManager *NodegroupManager

	infra   *Infrastructure
	cluster *Cluster

	initTime time.Time
}

type deployerOptions struct {
	AMI                         string        `flag:"ami" desc:"AMI for nodes"`
	ClusterRoleServicePrincipal string        `flag:"cluster-role-service-principal" desc:"Additional service principal that can assume the cluster role"`
	EKSEndpointURL              string        `flag:"endpoint-url" desc:"Endpoint URL for the EKS API"`
	EmitMetrics                 bool          `flag:"emit-metrics" desc:"Record and emit metrics to CloudWatch"`
	ExpectedAMI                 string        `flag:"expected-ami" desc:"Expected AMI of nodes. Up will fail if the actual nodes are not utilizing the expected AMI. Defaults to --ami if defined."`
	GenerateSSHKey              bool          `flag:"generate-ssh-key" desc:"Generate an SSH key to use for tests. The generated key should not be used in production, as it will not have a passphrase."`
	InstanceTypes               []string      `flag:"instance-types" desc:"Node instance types"`
	IPFamily                    string        `flag:"ip-family" desc:"IP family for the cluster (ipv4 or ipv6)"`
	KubeconfigPath              string        `flag:"kubeconfig" desc:"Path to kubeconfig"`
	KubernetesVersion           string        `flag:"kubernetes-version" desc:"cluster Kubernetes version"`
	NodeReadyTimeout            time.Duration `flag:"node-ready-timeout" desc:"Time to wait for all nodes to become ready"`
	Nodes                       int           `flag:"nodes" desc:"number of nodes to launch in cluster"`
	OsDistro                    string        `flag:"os-distro" desc:"Specifies the OS distribution for the AMI. Allowed values: ['al2', 'al2023'] (case-insensitive)"`
	Region                      string        `flag:"region" desc:"AWS region for EKS cluster"`
	UnmanagedNodes              bool          `flag:"unmanaged-nodes" desc:"Use an AutoScalingGroup instead of an EKS-managed nodegroup."`
	UpClusterHeaders            []string      `flag:"up-cluster-header" desc:"Additional header to add to eks:CreateCluster requests. Specified in the same format as curl's -H flag."`
	UserDataFormat              string        `flag:"user-data-format" desc:"Format of the node instance user data"`
}

// NewDeployer implements deployer.New for EKS using the EKS (and other AWS) API(s) directly (no cloudformation)
func NewDeployer(opts types.Options) (types.Deployer, *pflag.FlagSet) {
	// create a deployer object and set fields that are not flag controlled
	d := &deployer{
		commonOptions: opts,
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

func (d *deployer) Init() error {
	d.initTime = time.Now()
	awsConfig := awssdk.NewConfig()
	awsClients := newAWSClients(awsConfig, d.EKSEndpointURL)
	resourceID := ResourcePrefix + "-" + d.commonOptions.RunID()
	if d.deployerOptions.EmitMetrics {
		client := cloudwatch.NewFromConfig(awsConfig)
		d.metrics = metrics.NewCloudWatchRegistry(client)
	} else {
		d.metrics = metrics.NewNoopMetricRegistry()
	}
	d.infraManager = NewInfrastructureManager(awsClients, resourceID, d.metrics)
	d.clusterManager = NewClusterManager(awsClients, resourceID)
	d.nodegroupManager = NewNodegroupManager(awsClients, resourceID)
	return nil
}

func (d *deployer) Finish() error {
	d.metrics.Record(totalRuntimeSeconds, float64(time.Since(d.initTime).Seconds()), nil)
	return d.metrics.Emit()
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

func (d *deployer) Up() error {
	if err := d.verifyUpFlags(); err != nil {
		return fmt.Errorf("up flags are invalid: %v", err)
	}
	if d.GenerateSSHKey {
		if err := generateSSHKey(); err != nil {
			return err
		}
	}
	if infra, err := d.infraManager.createInfrastructureStack(&d.deployerOptions); err != nil {
		return err
	} else {
		d.infra = infra
	}
	cluster, err := d.clusterManager.createCluster(d.infra, &d.deployerOptions)
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
		if err := createAWSAuthConfigMap(k8sClient, d.infra.nodeRole, d.OsDistro); err != nil {
			return err
		}
	}
	if d.AMI != "" && d.ExpectedAMI == "" {
		d.ExpectedAMI = d.AMI
	}
	if err := d.nodegroupManager.createNodegroup(d.infra, d.cluster, &d.deployerOptions); err != nil {
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
			"m6i.large",
			"m6a.large",
			"m5.large",
			"m5a.large",
			"m4.large",
		}
		klog.V(2).Infof("Using default instance types: %v", d.InstanceTypes)
	}
	if d.UnmanagedNodes && d.AMI == "" {
		return fmt.Errorf("--ami must be specified for --unmanaged-nodes")
	}
	//TODO: add support for Manage node group once it supports AL2023
	if d.UnmanagedNodes && d.OsDistro == "" {
		return fmt.Errorf("--os-distro must be specified for --unmanaged-nodes")
	}
	if d.UnmanagedNodes && !util.IsStringInSlice(d.OsDistro, SupportedOsDistro) {
		return fmt.Errorf("--os-distro must be one of the following values: ['al2', 'al2023'] (case-insensitive)")
	}
	if d.UnmanagedNodes && d.UserDataFormat == "" {
		d.UserDataFormat = "bootstrap.sh"
		klog.V(2).Infof("Using default user data format: %s", d.UserDataFormat)
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
	return d.clusterManager.isClusterActive()
}

func (d *deployer) Down() error {
	return deleteResources(d.infraManager, d.clusterManager, d.nodegroupManager)
}

func deleteResources(im *InfrastructureManager, cm *ClusterManager, nm *NodegroupManager) error {
	if err := nm.deleteNodegroup(); err != nil {
		return err
	}
	// the EKS-managed cluster security group may be associated with a leaked ENI
	// so we need to make sure we've deleted leaked ENIs before we delete the cluster
	// otherwise, the cluster security group will be left behind and will block deletion of our VPC
	if err := im.deleteLeakedENIs(); err != nil {
		return err
	}
	if err := cm.deleteCluster(); err != nil {
		return err
	}
	return im.deleteInfrastructureStack()
}
