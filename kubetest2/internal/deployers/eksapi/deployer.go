package eksapi

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/kubetest2/internal"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/awssdk"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/metrics"
	"github.com/aws/aws-k8s-tester/kubetest2/internal/util"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/octago/sflags/gen/gpflag"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
	"k8s.io/klog"
	"sigs.k8s.io/kubetest2/pkg/types"
)

// DeployerName is the name of the deployer
const DeployerName = "eksapi"

const ResourcePrefix = "kubetest2-" + DeployerName

var SupportedNodeNameStrategy = []string{"SessionName", "EC2PrivateDNSName"}

// assert that deployer implements optional interfaces
var _ types.DeployerWithKubeconfig = &deployer{}
var _ types.DeployerWithInit = &deployer{}
var _ types.DeployerWithFinish = &deployer{}

type deployer struct {
	commonOptions types.Options
	deployerOptions

	metrics              metrics.MetricRegistry
	infraManager         *InfrastructureManager
	clusterManager       *ClusterManager
	addonManager         *AddonManager
	nodeManager          *nodeManager
	logManager           *logManager
	staticClusterManager *StaticClusterManager

	awsClients *awsClients

	infra   *Infrastructure
	cluster *Cluster

	k8sClient *k8sClient

	initTime time.Time
}

type deployerOptions struct {
	Addons                      []string `flag:"addons" desc:"Managed addons (name:version pairs) to create in the cluster. Use 'latest' for the most recent version, or 'default' for the default version."`
	AMI                         string   `flag:"ami" desc:"AMI for unmanaged nodes"`
	AMIType                     string   `flag:"ami-type" desc:"AMI type for managed nodes"`
	AutoMode                    bool     `flag:"auto-mode" desc:"Enable EKS Auto Mode"`
	CapacityReservation         bool     `flag:"capacity-reservation" desc:"Use capacity reservation for the unmanaged nodegroup"`
	ClusterRoleServicePrincipal string   `flag:"cluster-role-service-principal" desc:"Additional service principal that can assume the cluster role"`
	EFA                         bool     `flag:"efa" desc:"Create EFA interfaces on the node of an unmanaged nodegroup. Requires --unmanaged-nodes."`
	EKSEndpointURL              string   `flag:"endpoint-url" desc:"Endpoint URL for the EKS API"`
	EmitMetrics                 bool     `flag:"emit-metrics" desc:"Record and emit metrics to CloudWatch"`
	ExpectedAMI                 string   `flag:"expected-ami" desc:"Expected AMI of nodes. Up will fail if the actual nodes are not utilizing the expected AMI. Defaults to --ami if defined."`
	// TODO: remove this once it's no longer used in downstream jobs
	GenerateSSHKey      bool          `flag:"generate-ssh-key" desc:"Generate an SSH key to use for tests. The generated key should not be used in production, as it will not have a passphrase."`
	InstanceTypes       []string      `flag:"instance-types" desc:"Node instance types. Cannot be used with --instance-type-archs"`
	InstanceTypeArchs   []string      `flag:"instance-type-archs" desc:"Use default node instance types for specific architectures. Cannot be used with --instance-types"`
	IPFamily            string        `flag:"ip-family" desc:"IP family for the cluster (ipv4 or ipv6)"`
	KubeconfigPath      string        `flag:"kubeconfig" desc:"Path to kubeconfig"`
	KubernetesVersion   string        `flag:"kubernetes-version" desc:"cluster Kubernetes version"`
	LogBucket           string        `flag:"log-bucket" desc:"S3 bucket for storing logs for each run. If empty, logs will not be stored."`
	NodeCreationTimeout time.Duration `flag:"node-creation-timeout" desc:"Time to wait for nodes to be created/launched. This should consider instance availability."`
	NodeReadyTimeout    time.Duration `flag:"node-ready-timeout" desc:"Time to wait for all nodes to become ready"`
	Nodes               int           `flag:"nodes" desc:"number of nodes to launch in cluster"`
	NodeNameStrategy    string        `flag:"node-name-strategy" desc:"Specifies the naming strategy for node. Allowed values: ['SessionName', 'EC2PrivateDNSName'], default to EC2PrivateDNSName"`
	Region              string        `flag:"region" desc:"AWS region for EKS cluster"`
	StaticClusterName   string        `flag:"static-cluster-name" desc:"Optional when re-use existing cluster and node group by querying the kubeconfig and run test"`
	TuneVPCCNI          bool          `flag:"tune-vpc-cni" desc:"Apply tuning parameters to the VPC CNI DaemonSet"`
	UnmanagedNodes      bool          `flag:"unmanaged-nodes" desc:"Use an AutoScalingGroup instead of an EKS-managed nodegroup. Requires --ami"`
	UpClusterHeaders    []string      `flag:"up-cluster-header" desc:"Additional header to add to eks:CreateCluster requests. Specified in the same format as curl's -H flag."`
	UserDataFormat      string        `flag:"user-data-format" desc:"Format of the node instance user data"`
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
	d.awsClients = newAWSClients(awsConfig, d.EKSEndpointURL)
	resourceID := ResourcePrefix + "-" + d.commonOptions.RunID()
	if d.deployerOptions.EmitMetrics {
		client := cloudwatch.NewFromConfig(awsConfig)
		d.metrics = metrics.NewCloudWatchRegistry(client)
	} else {
		d.metrics = metrics.NewNoopMetricRegistry()
	}
	d.infraManager = NewInfrastructureManager(d.awsClients, resourceID, d.metrics)
	d.clusterManager = NewClusterManager(d.awsClients, resourceID)
	d.addonManager = NewAddonManager(d.awsClients)
	d.nodeManager = NewNodeManager(d.awsClients, resourceID)
	d.logManager = NewLogManager(d.awsClients, resourceID)
	if d.deployerOptions.StaticClusterName != "" {
		d.staticClusterManager = NewStaticClusterManager(&d.deployerOptions)
	}
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
	if d.deployerOptions.StaticClusterName == "" {
		if infra, err := d.infraManager.createInfrastructureStack(&d.deployerOptions); err != nil {
			return err
		} else {
			d.infra = infra
		}
	}
	cluster, err := d.clusterManager.getOrCreateCluster(d.infra, &d.deployerOptions)
	if err != nil {
		return err
	}
	d.cluster = cluster
	kubeconfig, err := d.Kubeconfig()
	if err != nil {
		return err
	}
	d.k8sClient, err = newK8sClient(kubeconfig)
	if err != nil {
		return err
	}
	if d.deployerOptions.StaticClusterName != "" {
		klog.Infof("inited k8sclient, skip the rest resource creation for static cluster")
		d.staticClusterManager.SetK8sClient(kubeconfig)
		if err := d.staticClusterManager.EnsureNodeForStaticCluster(); err != nil {
			klog.Errorf("Failed to launch nodes: %v", err)
			return err
		}
		klog.Infof("Nodes launched for static cluster")
		return nil
	}
	if d.UnmanagedNodes {
		if err := d.k8sClient.createAWSAuthConfigMap(d.NodeNameStrategy, d.infra.nodeRoleARN); err != nil {
			return err
		}
	}
	if d.AMI != "" && d.ExpectedAMI == "" {
		d.ExpectedAMI = d.AMI
	}
	if err := d.addonManager.createAddons(d.infra, d.cluster, &d.deployerOptions); err != nil {
		return err
	}
	if d.deployerOptions.TuneVPCCNI {
		if err := d.k8sClient.tuneVPCCNI(); err != nil {
			return err
		}
	}
	if err := d.nodeManager.createNodes(d.infra, d.cluster, &d.deployerOptions, d.k8sClient); err != nil {
		return err
	}
	if err := d.k8sClient.waitForReadyNodes(d.Nodes, d.NodeReadyTimeout); err != nil {
		return err
	}
	if d.EmitMetrics {
		if err := d.k8sClient.emitNodeMetrics(d.metrics, d.awsClients.EC2()); err != nil {
			return err
		}
	}
	if err := d.logManager.gatherLogsFromNodes(d.k8sClient, &d.deployerOptions, deployerPhaseUp); err != nil {
		klog.Warningf("failed to gather logs from nodes: %v", err)
		// don't return err, this isn't critical
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
		klog.Infof("Using default number of nodes: %d", d.Nodes)
	}
	if d.IPFamily == "" {
		d.IPFamily = string(ekstypes.IpFamilyIpv4)
		klog.Infof("Using default IP family: %s", d.IPFamily)
	}
	if d.NodeCreationTimeout == 0 {
		d.NodeCreationTimeout = time.Minute * 20
	}
	if d.NodeReadyTimeout == 0 {
		d.NodeReadyTimeout = time.Minute * 5
	}
	if d.StaticClusterName != "" {
		klog.Infof("Skip configuration for static cluster")
		return nil
	}
	if len(d.InstanceTypes) > 0 && len(d.InstanceTypeArchs) > 0 {
		return fmt.Errorf("--instance-types and --instance-type-archs are mutually exclusive")
	}
	if d.UnmanagedNodes {
		if d.AMI == "" {
			return fmt.Errorf("--ami must be specified for --unmanaged-nodes")
		}
		if d.AMIType != "" {
			return fmt.Errorf("--ami-type should not be provided with --unmanaged-nodes")
		}
		if d.NodeNameStrategy == "" {
			d.NodeNameStrategy = "EC2PrivateDNSName"
			klog.Infof("Using default node name strategy: EC2PrivateDNSName")
		} else {
			if !slices.Contains(SupportedNodeNameStrategy, d.NodeNameStrategy) {
				return fmt.Errorf("--node-name-strategy must be one of the following values: ['SessionName', 'EC2PrivateDNSName']")
			}
		}
		if d.UserDataFormat == "" {
			d.UserDataFormat = "bootstrap.sh"
			klog.Infof("Using default user data format: %s", d.UserDataFormat)
		}
		if d.EFA && len(d.InstanceTypes) != 1 {
			return fmt.Errorf("--efa requires a single instance type")
		}
	} else {
		if d.AMI != "" {
			return fmt.Errorf("--ami should not be provided without --unmanaged-nodes")
		}
		if d.AMIType == "" {
			d.AMIType = "AL2023_x86_64_STANDARD"
			klog.Infof("Using default AMI type: %s", d.AMIType)
		}
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
	if err := d.logManager.gatherLogsFromNodes(d.k8sClient, &d.deployerOptions, deployerPhaseDown); err != nil {
		klog.Warningf("failed to gather logs from nodes: %v", err)
		// don't return err, this isn't critical
	}
	if d.deployerOptions.StaticClusterName != "" {
		return d.staticClusterManager.TearDownNodeForStaticCluster()
	}
	return deleteResources(d.infraManager, d.clusterManager, d.nodeManager, d.k8sClient)
}

func deleteResources(im *InfrastructureManager, cm *ClusterManager, nm *nodeManager /* nillable */, k8sClient *k8sClient) error {
	if err := nm.deleteNodes(k8sClient); err != nil {
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
