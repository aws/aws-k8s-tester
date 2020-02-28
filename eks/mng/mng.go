// Package mng defines AWS EKS Managed Node Group configuration.
package mng

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	testerec2 "github.com/aws/aws-k8s-tester/ec2"
	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eksconfig"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	awsapiec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// MAKE SURE TO SYNC THE DEFAULT VALUES in "eksconfig"

// TemplateManagedNodeGroupWithNoReleaseVersion is the CloudFormation template for EKS managed node group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateManagedNodeGroupWithNoReleaseVersion = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Managed Node Group'

Parameters:

  ClusterName:
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.
    Type: String

  ManagedNodeGroupRoleARN:
    Description: The ARN of the node instance role
    Type: String

  ManagedNodeGroupName:
    Description: Unique identifier for the Node Group.
    Type: String

  PublicSubnetIDs:
    Description: The public subnet IDs where workers can be created.
    Type: List<AWS::EC2::Subnet::Id>

  ManagedNodeGroupSSHKeyPairName:
    Description: Amazon EC2 Key Pair
    Type: AWS::EC2::KeyPair::KeyName

  ManagedNodeGroupAMIType:
    Description: AMI type for your node group
    Type: String
    Default: AL2_x86_64
    AllowedValues:
    - AL2_x86_64
    - AL2_x86_64_GPU

  ManagedNodeGroupASGMinSize:
    Description: Minimum size of Node Group Auto Scaling Group.
    Type: Number
    Default: 3

  ManagedNodeGroupASGMaxSize:
    Description: Maximum size of Node Group Auto Scaling Group. Set to at least 1 greater than ManagedNodeGroupASGDesiredCapacity.
    Type: Number
    Default: 3

  ManagedNodeGroupASGDesiredCapacity:
    Description: Desired capacity of Node Group Auto Scaling Group.
    Type: Number
    Default: 3

  ManagedNodeGroupInstanceTypes:
    Description: EC2 instance types for the node instances
    Type: CommaDelimitedList
    Default: c5.xlarge

  ManagedNodeGroupVolumeSize:
    Description: Node volume size
    Type: Number
    Default: 40

Resources:

  ManagedNodeGroup:
    Type: AWS::EKS::Nodegroup
    Properties:
      ClusterName: !Ref ClusterName
      NodegroupName: !Ref ManagedNodeGroupName
      NodeRole: !Ref ManagedNodeGroupRoleARN
      AmiType: !Ref ManagedNodeGroupAMIType
      DiskSize: !Ref ManagedNodeGroupVolumeSize
      InstanceTypes: !Ref ManagedNodeGroupInstanceTypes
      RemoteAccess:
        Ec2SshKey: !Ref ManagedNodeGroupSSHKeyPairName
      ScalingConfig:
        DesiredSize: !Ref ManagedNodeGroupASGDesiredCapacity
        MinSize: !Ref ManagedNodeGroupASGMinSize
        MaxSize: !Ref ManagedNodeGroupASGMaxSize
      Subnets: !Ref PublicSubnetIDs
      Labels:
        Name: !Ref ManagedNodeGroupName

Outputs:

  ManagedNodeGroupID:
    Description: The managed node group resource Physical ID
    Value: !Ref ManagedNodeGroup

`

// TemplateManagedNodeGroupWithReleaseVersion is the CloudFormation template for EKS managed node group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateManagedNodeGroupWithReleaseVersion = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Managed Node Group'

Parameters:

  ClusterName:
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.
    Type: String

  ManagedNodeGroupRoleARN:
    Description: The ARN of the node instance role
    Type: String

  ManagedNodeGroupName:
    Description: Unique identifier for the Node Group.
    Type: String

  PublicSubnetIDs:
    Description: The private subnet IDs where workers can be created.
    Type: List<AWS::EC2::Subnet::Id>

  ManagedNodeGroupSSHKeyPairName:
    Description: Amazon EC2 Key Pair
    Type: AWS::EC2::KeyPair::KeyName

  ManagedNodeGroupAMIType:
    Description: AMI type for your node group
    Type: String
    Default: AL2_x86_64
    AllowedValues:
    - AL2_x86_64
    - AL2_x86_64_GPU

  ManagedNodeGroupASGMinSize:
    Description: Minimum size of Node Group Auto Scaling Group.
    Type: Number
    Default: 3

  ManagedNodeGroupASGMaxSize:
    Description: Maximum size of Node Group Auto Scaling Group. Set to at least 1 greater than ManagedNodeGroupASGDesiredCapacity.
    Type: Number
    Default: 3

  ManagedNodeGroupASGDesiredCapacity:
    Description: Desired capacity of Node Group Auto Scaling Group.
    Type: Number
    Default: 3

  ManagedNodeGroupInstanceTypes:
    Description: EC2 instance types for the node instances
    Type: CommaDelimitedList
    Default: c5.xlarge

  ManagedNodeGroupVolumeSize:
    Description: Node volume size
    Type: Number
    Default: 40

  ManagedNodeGroupReleaseVersion:
    Description: AMI version of the Amazon EKS-optimized AMI
    Type: String

Resources:

  ManagedNodeGroup:
    Type: AWS::EKS::Nodegroup
    Properties:
      ClusterName: !Ref ClusterName
      NodegroupName: !Ref ManagedNodeGroupName
      NodeRole: !Ref ManagedNodeGroupRoleARN
      AmiType: !Ref ManagedNodeGroupAMIType
      DiskSize: !Ref ManagedNodeGroupVolumeSize
      InstanceTypes: !Ref ManagedNodeGroupInstanceTypes
      ReleaseVersion: !Ref ManagedNodeGroupReleaseVersion
      RemoteAccess:
        Ec2SshKey: !Ref ManagedNodeGroupSSHKeyPairName
      ScalingConfig:
        DesiredSize: !Ref ManagedNodeGroupASGDesiredCapacity
        MinSize: !Ref ManagedNodeGroupASGMinSize
        MaxSize: !Ref ManagedNodeGroupASGMaxSize
      Subnets: !Ref PublicSubnetIDs
      Labels:
        Name: !Ref ManagedNodeGroupName

Outputs:

  ManagedNodeGroupID:
    Description: The managed node group resource Physical ID
    Value: !Ref ManagedNodeGroup

`

// Config defines Managed Node Group configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	CFNAPI    cloudformationiface.CloudFormationAPI
	EC2API    ec2iface.EC2API
	ASGAPI    autoscalingiface.AutoScalingAPI
	EKSAPI    eksiface.EKSAPI
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester implements EKS "Managed Node Group" for "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type Tester interface {
	// Create creates EKS "Managed Node Group", and waits for completion.
	Create() error
	// Delete deletes all EKS "Managed Node Group" resources.
	Delete() error

	// FetchLogs fetches logs from all worker nodes.
	FetchLogs() error
	// DownloadClusterLogs dumps all logs to artifact directory.
	// Let default kubetest log dumper handle all artifact uploads.
	// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
	DownloadClusterLogs(artifactDir string) error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{
		cfg:    cfg,
		logsMu: new(sync.RWMutex),
	}, nil
}

type tester struct {
	cfg        Config
	logsMu     *sync.RWMutex
	failedOnce bool
}

func (ts *tester) Create() (err error) {
	if len(ts.cfg.EKSConfig.Status.PublicSubnetIDs) == 0 {
		return errors.New("empty EKSConfig.Status.PublicSubnetIDs")
	}

	if err = ts.createKeyPair(); err != nil {
		return err
	}
	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createMNG(); err != nil {
		return err
	}
	for name := range ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes {
		if err = ts.openPorts(name); err != nil {
			return err
		}
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	var errs []string
	if err := ts.deleteKeyPair(); err != nil {
		ts.cfg.Logger.Warn("failed to delete key pair", zap.Error(err))
		errs = append(errs, err.Error())
	}

	var err error
	for i := 0; i < 5; i++ { // retry, leakly ENI may take awhile to be deleted
		err = ts.deleteMNG()
		if err == nil {
			break
		}
		ts.failedOnce = true
		ts.cfg.Logger.Warn("failed to delete mng", zap.Error(err))
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("aborted")
			return nil
		case <-time.After(time.Minute):
		}
	}
	if err != nil {
		errs = append(errs, err.Error())
	}

	waitDur := 5 * time.Second
	ts.cfg.Logger.Info("sleeping before node group role deletion", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	// must be run after deleting node group
	// otherwise, "Cannot delete entity, must remove roles from instance profile first. (Service: AmazonIdentityManagement; Status Code: 409; Error Code: DeleteConflict; Request ID: 197f795b-1003-4386-81cc-44a926c42be7)"
	if err := ts.deleteRole(); err != nil {
		ts.cfg.Logger.Warn("failed to delete mng role", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createMNG() error {
	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.StatusManagedNodeGroups.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.StatusManagedNodeGroups.CreateTookString = ts.cfg.EKSConfig.StatusManagedNodeGroups.CreateTook.String()
	}()

	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName == "" {
		return errors.New("empty EKSConfig.AddOnManagedNodeGroups.RoleName")
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN == "" {
		return errors.New("empty AddOnManagedNodeGroups.RoleARN")
	}
	if ts.cfg.EKSConfig.StatusManagedNodeGroups.RoleCFNStackID == "" {
		return errors.New("empty StatusManagedNodeGroups.RoleCFNStackID")
	}

	// need use EKS API directly for beta
	// otherwise,
	// No cluster found for name: BETA_CLUSTER_NAME.
	// (Service: AmazonEKS; Status Code: 404; Error Code: ResourceNotFoundException; Request ID: FOO)

	ts.cfg.Logger.Info("creating managed node groups",
		zap.String("resolver-url", ts.cfg.EKSConfig.AddOnManagedNodeGroups.ResolverURL),
		zap.String("signing-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SigningName),
		zap.String("request-header-key", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey),
		zap.String("request-header-value", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue),
	)

	now := time.Now()
	initialWait := 3 * time.Minute
	cfnStacks := make(map[string]string)

	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.ResolverURL != "" ||
		(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey != "" &&
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue != "") {

		for k, mv := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			vv, ok := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[k]
			if ok && (vv.CreateRequested || vv.CFNStackID != "") {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("name", k),
					zap.Bool("create-requested", vv.CreateRequested),
					zap.String("cfn-stack-id", vv.CFNStackID),
				)
				continue
			}

			ts.cfg.Logger.Info("creating a managed node group using EKS API", zap.String("name", mv.Name))
			createInput := awseks.CreateNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(mv.Name),
				NodeRole:      aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
				AmiType:       aws.String(mv.AMIType),
				DiskSize:      aws.Int64(int64(mv.VolumeSize)),
				InstanceTypes: aws.StringSlice(mv.InstanceTypes),
				RemoteAccess: &awseks.RemoteAccessConfig{
					Ec2SshKey: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName),
				},
				ScalingConfig: &awseks.NodegroupScalingConfig{
					DesiredSize: aws.Int64(int64(mv.ASGDesiredCapacity)),
					MinSize:     aws.Int64(int64(mv.ASGMinSize)),
					MaxSize:     aws.Int64(int64(mv.ASGMaxSize)),
				},
				Subnets: aws.StringSlice(ts.cfg.EKSConfig.Status.PublicSubnetIDs),
				Tags: map[string]*string{
					"Kind": aws.String("aws-k8s-tester"),
				},
				Labels: map[string]*string{
					"Name": aws.String(mv.Name),
				},
			}
			for k, v := range mv.Tags {
				createInput.Tags[k] = aws.String(v)
				ts.cfg.Logger.Info("added EKS tag", zap.String("key", k), zap.String("value", v))
			}
			if mv.ReleaseVersion != "" {
				createInput.ReleaseVersion = aws.String(mv.ReleaseVersion)
				ts.cfg.Logger.Info("added EKS release version", zap.String("version", mv.ReleaseVersion))
			}

			req, _ := ts.cfg.EKSAPI.CreateNodegroupRequest(&createInput)
			if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey != "" && ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue != "" {
				req.HTTPRequest.Header[ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey] = []string{ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue}
				ts.cfg.Logger.Info("set request header for EKS managed node group create request",
					zap.String("key", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey),
					zap.String("value", ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue),
				)
			}

			err := req.Send()
			if err != nil {
				ts.cfg.Logger.Warn("failed to created MNG", zap.Error(err))
				return fmt.Errorf("create node group request failed (%v)", err)
			}

			ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[mv.Name] = eksconfig.StatusManagedNodeGroup{
				CreateRequested: true,
				Status:          awseks.NodegroupStatusCreating,
				Logs:            make(map[string][]string),
			}
			ts.cfg.EKSConfig.Sync()
			ts.cfg.Logger.Info("sent create managed node group request")

			// when used with EKS API directly, just use "Poll" below to sync status
		}

	} else {

		// already wait here, reduce initial wait when created with CFN
		initialWait = 30 * time.Second

		for k, mv := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			vv, ok := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[k]
			if ok && (vv.CreateRequested || vv.CFNStackID != "") {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("name", k),
					zap.Bool("create-requested", vv.CreateRequested),
					zap.String("cfn-stack-id", vv.CFNStackID),
				)
				continue
			}

			ts.cfg.Logger.Info("creating a new node group using CFN", zap.String("name", mv.Name))
			stackInput := &cloudformation.CreateStackInput{
				StackName:    aws.String(mv.Name),
				Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
				OnFailure:    aws.String(cloudformation.OnFailureDelete),
				Tags: awscfn.NewTags(map[string]string{
					"Kind": "aws-k8s-tester",
					"Name": ts.cfg.EKSConfig.Name,
				}),
				Parameters: []*cloudformation.Parameter{
					{
						ParameterKey:   aws.String("ClusterName"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.Name),
					},
					{
						ParameterKey:   aws.String("ManagedNodeGroupRoleARN"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
					},
					{
						ParameterKey:   aws.String("ManagedNodeGroupName"),
						ParameterValue: aws.String(mv.Name),
					},
					{
						ParameterKey:   aws.String("PublicSubnetIDs"),
						ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.Status.PublicSubnetIDs, ",")),
					},
					{
						ParameterKey:   aws.String("ManagedNodeGroupSSHKeyPairName"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName),
					},
				},
			}
			if mv.ReleaseVersion == "" {
				stackInput.TemplateBody = aws.String(TemplateManagedNodeGroupWithNoReleaseVersion)
			} else {
				stackInput.TemplateBody = aws.String(TemplateManagedNodeGroupWithReleaseVersion)
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupReleaseVersion"),
					ParameterValue: aws.String(mv.ReleaseVersion),
				})
			}
			if mv.AMIType != "" {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupAMIType"),
					ParameterValue: aws.String(mv.AMIType),
				})
			}
			if mv.ASGMinSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupASGMinSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", mv.ASGMinSize)),
				})
			}
			if mv.ASGMaxSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupASGMaxSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", mv.ASGMaxSize)),
				})
			}
			if mv.ASGDesiredCapacity > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupASGDesiredCapacity"),
					ParameterValue: aws.String(fmt.Sprintf("%d", mv.ASGDesiredCapacity)),
				})
			}
			if len(mv.InstanceTypes) > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupInstanceTypes"),
					ParameterValue: aws.String(strings.Join(mv.InstanceTypes, ",")),
				})
			}
			if mv.VolumeSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ManagedNodeGroupVolumeSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", mv.VolumeSize)),
				})
			}

			stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
			if err != nil {
				return err
			}

			stackID := aws.StringValue(stackOutput.StackId)
			cfnStacks[mv.Name] = stackID
			ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[mv.Name] = eksconfig.StatusManagedNodeGroup{
				CreateRequested: true,
				CFNStackID:      stackID,
				Status:          cloudformation.ResourceStatusCreateInProgress,
				Logs:            make(map[string][]string),
			}
			ts.cfg.EKSConfig.Sync()
		}

		cfnInitWait, cnt := 2*time.Minute, 0
		for name, stackID := range cfnStacks {
			cfnInitWait -= time.Duration(cnt*30) * time.Second
			if cfnInitWait < 0 {
				cfnInitWait = 10 * time.Second
			}
			status := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name]
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			ch := awscfn.Poll(
				ctx,
				ts.cfg.Stopc,
				ts.cfg.Sig,
				ts.cfg.Logger,
				ts.cfg.CFNAPI,
				stackID,
				cloudformation.ResourceStatusCreateComplete,
				cfnInitWait,
				15*time.Second,
			)
			var st awscfn.StackStatus
			for st = range ch {
				if st.Error != nil {
					status.Status = fmt.Sprintf("failed to create managed node group (%v)", st.Error)
					ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = status
					ts.cfg.EKSConfig.Sync()
				}
			}
			cancel()

			// update status after creating a new managed node group CFN
			for _, o := range st.Stack.Outputs {
				switch k := aws.StringValue(o.OutputKey); k {
				case "ManagedNodeGroupID":
					status.PhysicalID = aws.StringValue(o.OutputValue)
					ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = status
					ts.cfg.EKSConfig.Sync()
				default:
					return fmt.Errorf("unexpected OutputKey %q from %q", k, stackID)
				}
			}
		}

	}

	for _, v := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			v.Name,
			awseks.NodegroupStatusActive,
			initialWait,
			20*time.Second,
		)
		for sv := range ch {
			if serr := ts.setStatus(sv); serr != nil {
				cancel()
				return serr
			}
		}
		cancel()
		if err := ts.waitForNodes(v.Name); err != nil {
			return err
		}
		ts.cfg.Logger.Info("created a managed node group",
			zap.String("mng-name", v.Name),
			zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteMNG() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.StatusManagedNodeGroups.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.StatusManagedNodeGroups.DeleteTookString = ts.cfg.EKSConfig.StatusManagedNodeGroups.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting managed node groups")
	for name, mv := range ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes {
		if name == "" {
			ts.cfg.Logger.Warn("empty name found in status map")
			delete(ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes, "")
			continue
		}
		if mv.Status == "" || mv.Status == ManagedNodeGroupStatusDELETEDORNOTEXIST {
			ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", name))
			continue
		}

		useCFN := mv.CFNStackID != ""
		if ts.failedOnce {
			useCFN = false
		}

		if useCFN {
			ts.cfg.Logger.Info("deleting managed node group using CFN", zap.String("name", name), zap.String("cfn-stack-id", name))
			_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: aws.String(mv.CFNStackID),
			})
			if err != nil {
				return err
			}
			initialWait, timeout := 2*time.Minute, 15*time.Minute
			if len(mv.Instances) > 50 {
				initialWait, timeout = 3*time.Minute, 20*time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ch := awscfn.Poll(
				ctx,
				make(chan struct{}),  // do not exit on stop
				make(chan os.Signal), // do not exit on stop
				ts.cfg.Logger,
				ts.cfg.CFNAPI,
				mv.CFNStackID,
				cloudformation.ResourceStatusDeleteComplete,
				initialWait,
				15*time.Second,
			)
			var st awscfn.StackStatus
			for st = range ch {
				if st.Error != nil {
					cancel()
					mv.Status = fmt.Sprintf("failed to delete a managed node group (%v)", st.Error)
					ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = mv
					ts.cfg.EKSConfig.Sync()
					ts.cfg.Logger.Error("polling errror", zap.Error(st.Error))
				}
			}
			cancel()
			if st.Error != nil {
				return st.Error
			}
			mv.Status = ManagedNodeGroupStatusDELETEDORNOTEXIST
			ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = mv
			ts.cfg.EKSConfig.Sync()

		} else {

			ts.cfg.Logger.Info("deleting managed node group using EKS API", zap.String("name", name))
			_, err := ts.cfg.EKSAPI.DeleteNodegroup(&awseks.DeleteNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(name),
			})
			if err != nil {
				mv.Status = fmt.Sprintf("failed to delete managed node group (%v)", err)
				ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = mv
				ts.cfg.EKSConfig.Sync()
				return err
			}

			initialWait, timeout := 2*time.Minute, 15*time.Minute
			if len(mv.Instances) > 50 {
				initialWait, timeout = 3*time.Minute, 20*time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ch := Poll(
				ctx,
				ts.cfg.Stopc,
				ts.cfg.Logger,
				ts.cfg.EKSAPI,
				ts.cfg.EKSConfig.Name,
				name,
				ManagedNodeGroupStatusDELETEDORNOTEXIST,
				initialWait,
				20*time.Second,
			)
			for v := range ch {
				if serr := ts.setStatus(v); err != nil {
					cancel()
					return serr
				}
			}
			cancel()
		}
	}

	ts.cfg.Logger.Info("deleted managed node groups")
	return ts.cfg.EKSConfig.Sync()
}

// ManagedNodeGroupStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
//
//  CREATING
//  ACTIVE
//  DELETING
//  FAILED
//  UPDATING
//
const ManagedNodeGroupStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// ManagedNodeGroupStatus represents the CloudFormation status.
type ManagedNodeGroupStatus struct {
	NodeGroupName string
	NodeGroup     *awseks.Nodegroup
	Error         error
}

// Poll periodically fetches the managed node group status
// until the node group becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	nodeGroupName string,
	desiredNodeGroupStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan ManagedNodeGroupStatus {
	lg.Info("polling mng",
		zap.String("cluster-name", clusterName),
		zap.String("mng-name", nodeGroupName),
		zap.String("desired-mng-status", desiredNodeGroupStatus),
	)

	now := time.Now()

	ch := make(chan ManagedNodeGroupStatus, 10)
	go func() {
		ticker := time.NewTicker(wait)
		defer ticker.Stop()

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-ticker.C:
			}

			output, err := eksAPI.DescribeNodegroup(&awseks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(nodeGroupName),
			})
			if err != nil {
				if IsDeleted(err) {
					if desiredNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
						lg.Info("managed node group is already deleted as desired; exiting", zap.Error(err))
						ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("managed node group does not exist", zap.Error(err))
					lg.Warn("aborting", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe managed node group failed; retrying", zap.Error(err))
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: err}
				continue
			}

			if output.Nodegroup == nil {
				lg.Warn("expected non-nil managed node group; retrying")
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			nodeGroup := output.Nodegroup
			currentStatus := aws.StringValue(nodeGroup.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("mng-name", nodeGroupName),
				zap.String("mng-status", currentStatus),
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredNodeGroupStatus:
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nodeGroup, Error: nil}
				lg.Info("became desired managed node group status; exiting", zap.String("status", currentStatus))
				close(ch)
				return
			case awseks.NodegroupStatusCreateFailed,
				awseks.NodegroupStatusDeleteFailed,
				awseks.NodegroupStatusDegraded:
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nodeGroup, Error: fmt.Errorf("unexpected mng status %q", currentStatus)}
				close(ch)
				return
			default:
				ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nodeGroup, Error: nil}
			}
			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ManagedNodeGroupStatus{NodeGroupName: nodeGroupName, NodeGroup: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

func (ts *tester) setStatus(sv ManagedNodeGroupStatus) error {
	name := sv.NodeGroupName
	if name == "" {
		return errors.New("EKS Managed Node Group empty name")
	}
	mv, ok := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name]
	if !ok {
		return fmt.Errorf("EKS Managed Node Group %q not found", name)
	}

	if sv.NodeGroup == nil {
		if sv.Error != nil {
			mv.Status = fmt.Sprintf("%q failed with error %v", sv.NodeGroupName, sv.Error)
		} else {
			mv.Status = ManagedNodeGroupStatusDELETEDORNOTEXIST
		}
	} else {
		mv.Status = aws.StringValue(sv.NodeGroup.Status)
		if sv.NodeGroup.Resources != nil {
			mv.RemoteAccessSecurityGroupID = aws.StringValue(sv.NodeGroup.Resources.RemoteAccessSecurityGroup)
		}
	}

	ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = mv
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitForNodes(name string) error {
	mv, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", name)
	}
	sv, ok := ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name]
	if !ok {
		return fmt.Errorf("Managed Node Group Status %q not found", name)
	}
	waitDur := 2*time.Minute + time.Duration(15*mv.ASGDesiredCapacity)*time.Second

	ts.cfg.Logger.Info("checking nodes via ASG", zap.String("mng-name", mv.Name))
	dout, err := ts.cfg.EKSAPI.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(mv.Name),
	})
	if err != nil {
		return err
	}
	sv.RemoteAccessSecurityGroupID = aws.StringValue(dout.Nodegroup.Resources.RemoteAccessSecurityGroup)
	ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = sv
	ts.cfg.EKSConfig.Sync()

	for _, asg := range dout.Nodegroup.Resources.AutoScalingGroups {
		asgName := aws.StringValue(asg.Name)

		var aout *autoscaling.DescribeAutoScalingGroupsOutput
		aout, err = ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgName}),
		})
		if err != nil {
			return fmt.Errorf("ASG %q not found (%v)", asgName, err)
		}
		if len(aout.AutoScalingGroups) != 1 {
			return fmt.Errorf("%q expected only 1 ASG, got %+v", asgName, aout.AutoScalingGroups)
		}

		av := aout.AutoScalingGroups[0]
		instanceIDs := make([]string, 0, len(av.Instances))
		for _, iv := range av.Instances {
			instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
		}

		ts.cfg.Logger.Info(
			"describing EC2 instances in ASG",
			zap.String("asg-name", asgName),
			zap.Strings("instance-ids", instanceIDs),
		)
		ec2Instances, err := awsapiec2.PollUntilRunning(
			waitDur,
			ts.cfg.Logger,
			ts.cfg.EC2API,
			instanceIDs...,
		)
		if err != nil {
			return err
		}
		sv, ok = ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name]
		if !ok {
			return fmt.Errorf("Managed Node Group Status %q not found", name)
		}
		sv.Instances = make(map[string]ec2config.Instance)
		for id, vv := range ec2Instances {
			sv.Instances[id] = testerec2.ConvertEC2Instance(vv)
		}
		ts.cfg.EKSConfig.StatusManagedNodeGroups.Nodes[name] = sv
		ts.cfg.EKSConfig.Sync()
	}

	var items []v1.Node
	ts.cfg.Logger.Info("checking nodes via client-go")
	retryStart, threshold := time.Now(), waitDur/10*7
	for time.Now().Sub(retryStart) < waitDur {
		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			ts.cfg.Logger.Error("get nodes failed", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		items = nodes.Items

		readies := 0
		for _, node := range items {
			for _, cond := range node.Status.Conditions {
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("node info",
					zap.String("name", node.GetName()),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				if cond.Status == v1.ConditionTrue {
					readies++
				}
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", mv.ASGDesiredCapacity),
		)
		if readies >= mv.ASGDesiredCapacity {
			break
		}
		took := time.Now().Sub(retryStart)
		if took > threshold {
			colorstring.Printf("\n\n[light_green]kubectl [default](%q, %q)\n\n", ts.cfg.EKSConfig.ConfigPath, ts.cfg.EKSConfig.KubectlCommand())
			fmt.Println(ts.cfg.EKSConfig.KubectlCommands())
			colorstring.Printf("\n\n[light_green]SSH [default](%q, %q)\n\n", ts.cfg.EKSConfig.ConfigPath, ts.cfg.EKSConfig.KubectlCommand())
			fmt.Println(ts.cfg.EKSConfig.SSHCommands())
		}
	}
	println()
	for _, v := range items {
		fmt.Printf("'Node' %q (using client-go): %+v\n", v.GetName(), v.Status.Addresses)
	}
	println()

	return ts.cfg.EKSConfig.Sync()
}

// IsDeleted returns true if error from EKS API indicates that
// the EKS managed node group has already been deleted.
func IsDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" {
		return true
	}

	// ResourceNotFoundException: nodeGroup eks-2019120505-pdx-us-west-2-tqy2d-managed-node-group not found for cluster eks-2019120505-pdx-us-west-2-tqy2d\n\tstatus code: 404, request id: 330998c1-22e9-4a8b-b180-420dadade090
	return strings.Contains(err.Error(), " not found for cluster ")
}
