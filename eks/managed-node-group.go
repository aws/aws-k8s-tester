package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eksconfig"
	internalec2 "github.com/aws/aws-k8s-tester/internal/ec2"
	awsapicfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
	awsapiec2 "github.com/aws/aws-k8s-tester/pkg/awsapi/ec2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/eks"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

// MAKE SURE TO SYNC THE DEFAULT VALUES in "eksconfig"

// TemplateManagedNodeGroup is the CloudFormation template for EKS managed node group.
// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateManagedNodeGroup = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Node Group Managed'

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

  PrivateSubnetIDs:
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
      Subnets: !Ref PrivateSubnetIDs
      Labels:
        Name: !Ref ManagedNodeGroupName

Outputs:

  ManagedNodeGroupID:
    Description: The managed node group resource Physical ID
    Value: !Ref ManagedNodeGroup

`

func (ts *Tester) createManagedNodeGroup() error {
	if ts.cfg.Parameters.ManagedNodeGroupRoleName == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRoleName")
	}
	if ts.cfg.Parameters.ManagedNodeGroupRoleARN == "" {
		return errors.New("empty Parameters.ManagedNodeGroupRoleARN")
	}
	if ts.cfg.Status.ManagedNodeGroupCFNStackID != "" {
		ts.lg.Info("non-empty node group stack given; no need to create a new one")
		return nil
	}

	// need use EKS API directly for beta
	// otherwise,
	// No cluster found for name: BETA_CLUSTER_NAME.
	// (Service: AmazonEKS; Status Code: 404; Error Code: ResourceNotFoundException; Request ID: FOO)

	now := time.Now().UTC()

	initialWait := 3 * time.Minute

	if ts.cfg.Parameters.ManagedNodeGroupResolverURL != "" || (ts.cfg.Parameters.ManagedNodeGroupRequestHeaderKey != "" && ts.cfg.Parameters.ManagedNodeGroupRequestHeaderValue != "") {
		ts.lg.Info("creating a managed node group using EKS API",
			zap.String("name", ts.cfg.Parameters.ManagedNodeGroupName),
			zap.String("resolver-url", ts.cfg.Parameters.ManagedNodeGroupResolverURL),
			zap.String("signing-name", ts.cfg.Parameters.ManagedNodeGroupSigningName),
			zap.String("request-header-key", ts.cfg.Parameters.ManagedNodeGroupRequestHeaderKey),
			zap.String("request-header-value", ts.cfg.Parameters.ManagedNodeGroupRequestHeaderValue),
		)
		createInput := awseks.CreateNodegroupInput{
			ClusterName:   aws.String(ts.cfg.Name),
			NodegroupName: aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
			NodeRole:      aws.String(ts.cfg.Parameters.ManagedNodeGroupRoleARN),
			AmiType:       aws.String(ts.cfg.Parameters.ManagedNodeGroupAMIType),
			DiskSize:      aws.Int64(int64(ts.cfg.Parameters.ManagedNodeGroupVolumeSize)),
			InstanceTypes: aws.StringSlice(ts.cfg.Parameters.ManagedNodeGroupInstanceTypes),
			RemoteAccess: &awseks.RemoteAccessConfig{
				Ec2SshKey: aws.String(ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName),
			},
			ScalingConfig: &awseks.NodegroupScalingConfig{
				DesiredSize: aws.Int64(int64(ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity)),
				MinSize:     aws.Int64(int64(ts.cfg.Parameters.ManagedNodeGroupASGMinSize)),
				MaxSize:     aws.Int64(int64(ts.cfg.Parameters.ManagedNodeGroupASGMaxSize)),
			},
			Subnets: aws.StringSlice(ts.cfg.Status.PrivateSubnetIDs),
			Tags: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
			},
			Labels: map[string]*string{
				"Name": aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
			},
		}
		for k, v := range ts.cfg.Parameters.ManagedNodeGroupTags {
			createInput.Tags[k] = aws.String(v)
			ts.lg.Info("added EKS tag", zap.String("key", k), zap.String("value", v))
		}
		req, _ := ts.eksAPI.CreateNodegroupRequest(&createInput)
		if ts.cfg.Parameters.ManagedNodeGroupRequestHeaderKey != "" && ts.cfg.Parameters.ManagedNodeGroupRequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.Parameters.ManagedNodeGroupRequestHeaderKey] = []string{ts.cfg.Parameters.ManagedNodeGroupRequestHeaderValue}
			ts.lg.Info("set request header for EKS managed node group create request",
				zap.String("key", ts.cfg.Parameters.ManagedNodeGroupRequestHeaderKey),
				zap.String("value", ts.cfg.Parameters.ManagedNodeGroupRequestHeaderValue),
			)
		}
		err := req.Send()
		if err != nil {
			return err
		}
		ts.lg.Info("sent create managed node group request")

	} else {

		initialWait = 30 * time.Second
		ts.lg.Info("creating a new node group using CFN", zap.String("name", ts.cfg.Parameters.ManagedNodeGroupName))
		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
			OnFailure:    aws.String("DELETE"),
			TemplateBody: aws.String(TemplateManagedNodeGroup),
			Tags: awsapicfn.NewTags(map[string]string{
				"Kind": "aws-k8s-tester",
				"Name": ts.cfg.Name,
			}),
			Parameters: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String("ClusterName"),
					ParameterValue: aws.String(ts.cfg.Name),
				},
				{
					ParameterKey:   aws.String("ManagedNodeGroupRoleARN"),
					ParameterValue: aws.String(ts.cfg.Parameters.ManagedNodeGroupRoleARN),
				},
				{
					ParameterKey:   aws.String("ManagedNodeGroupName"),
					ParameterValue: aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
				},
				{
					ParameterKey:   aws.String("PrivateSubnetIDs"),
					ParameterValue: aws.String(strings.Join(ts.cfg.Status.PrivateSubnetIDs, ",")),
				},
				{
					ParameterKey:   aws.String("ManagedNodeGroupSSHKeyPairName"),
					ParameterValue: aws.String(ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName),
				},
			},
		}
		if ts.cfg.Parameters.ManagedNodeGroupAMIType != "" {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupAMIType"),
				ParameterValue: aws.String(ts.cfg.Parameters.ManagedNodeGroupAMIType),
			})
		}
		if ts.cfg.Parameters.ManagedNodeGroupASGMinSize > 0 {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupASGMinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", ts.cfg.Parameters.ManagedNodeGroupASGMinSize)),
			})
		}
		if ts.cfg.Parameters.ManagedNodeGroupASGMaxSize > 0 {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupASGMaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", ts.cfg.Parameters.ManagedNodeGroupASGMaxSize)),
			})
		}
		if ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity > 0 {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupASGDesiredCapacity"),
				ParameterValue: aws.String(fmt.Sprintf("%d", ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity)),
			})
		}
		if len(ts.cfg.Parameters.ManagedNodeGroupInstanceTypes) > 0 {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupInstanceTypes"),
				ParameterValue: aws.String(strings.Join(ts.cfg.Parameters.ManagedNodeGroupInstanceTypes, ",")),
			})
		}
		if ts.cfg.Parameters.ManagedNodeGroupVolumeSize > 0 {
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("ManagedNodeGroupVolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", ts.cfg.Parameters.ManagedNodeGroupVolumeSize)),
			})
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		ts.cfg.Status.ManagedNodeGroupCFNStackID = aws.StringValue(stackOutput.StackId)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := awsapicfn.Poll(
			ctx,
			ts.stopCreationCh,
			ts.interruptSig,
			ts.lg,
			ts.cfnAPI,
			ts.cfg.Status.ManagedNodeGroupCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			2*time.Minute,
			15*time.Second,
		)
		var st awsapicfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.Status.ManagedNodeGroupStatus = fmt.Sprintf("failed to create managed node group (%v)", st.Error)
				ts.cfg.Sync()
			}
		}
		cancel()
		// update status after creating a new managed node group CFN
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "ManagedNodeGroupID":
				ts.cfg.Status.ManagedNodeGroupID = aws.StringValue(o.OutputValue)
			default:
				return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.Status.ManagedNodeGroupCFNStackID)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := PollEKSManagedNodeGroup(
		ctx,
		ts.stopCreationCh,
		ts.lg,
		ts.eksAPI,
		ts.cfg.Name,
		ts.cfg.Parameters.ManagedNodeGroupName,
		ManagedNodeGroupStatusACTIVE,
		initialWait,
		20*time.Second,
	)
	for v := range ch {
		ts.updateManagedNodeGroupStatus(v)
	}
	cancel()
	if err := ts.waitForNodes(); err != nil {
		return err
	}

	ts.lg.Info("created a managed node group",
		zap.String("managed-node-group-cfn-stack-id", ts.cfg.Status.ManagedNodeGroupCFNStackID),
		zap.String("managed-node-group-id", ts.cfg.Status.ManagedNodeGroupID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return ts.cfg.Sync()
}

// https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
//
//  CREATING
//  ACTIVE
//  DELETING
//  FAILED
//  UPDATING
//
const (
	ManagedNodeGroupStatusCREATING          = "CREATING"
	ManagedNodeGroupStatusACTIVE            = "ACTIVE"
	ManagedNodeGroupStatusUPDATING          = "UPDATING"
	ManagedNodeGroupStatusDELETING          = "DELETING"
	ManagedNodeGroupStatusCREATEFAILED      = "CREATE_FAILED"
	ManagedNodeGroupStatusUPDATEFAILED      = "UPDATE_FAILED"
	ManagedNodeGroupStatusDELETEFAILED      = "DELETE_FAILED"
	ManagedNodeGroupStatusDEGRADED          = "DEGRADED"
	ManagedNodeGroupStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"
)

// ManagedNodeGroupStatus represents the CloudFormation status.
type ManagedNodeGroupStatus struct {
	NodeGroup *awseks.Nodegroup
	Error     error
}

// PollEKSManagedNodeGroup periodically fetches the managed node group status
// until the node group becomes the desired state.
func PollEKSManagedNodeGroup(
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
	lg.Info("polling cluster",
		zap.String("cluster-name", clusterName),
		zap.String("managed-node-group-name", nodeGroupName),
		zap.String("desired-cluster-status", desiredNodeGroupStatus),
	)
	ch := make(chan ManagedNodeGroupStatus, 10)
	go func() {
		ticker := time.NewTicker(wait)
		defer ticker.Stop()

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-ticker.C:
			}

			output, err := eksAPI.DescribeNodegroup(&awseks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(nodeGroupName),
			})
			if err != nil {
				if ManangedNodeGroupDelete(err) {
					if desiredNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
						lg.Info("managed node group is already deleted as desired; exiting", zap.Error(err))
						ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("managed node group does not exist; aborting", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: err}
					close(ch)
					return
				}

				lg.Error("describe managed node group failed; retrying", zap.Error(err))
				ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: err}
				continue
			}

			if output.Nodegroup == nil {
				lg.Error("expected non-nil managed node group; retrying")
				ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			nodeGroup := output.Nodegroup
			status := aws.StringValue(nodeGroup.Status)

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				time.Sleep(initialWait)
				first = false
			}

			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("managed-node-group-name", nodeGroupName),
				zap.String("managed-node-group-status", status),
			)

			ch <- ManagedNodeGroupStatus{NodeGroup: nodeGroup, Error: nil}
			if status == desiredNodeGroupStatus {
				lg.Info("became desired managed node group status; exiting", zap.String("managed-node-group-status", status))
				close(ch)
				return
			}
			// continue for-loop
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ManagedNodeGroupStatus{NodeGroup: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

func (ts *Tester) updateManagedNodeGroupStatus(v ManagedNodeGroupStatus) {
	if v.NodeGroup == nil {
		if v.Error != nil {
			ts.cfg.Status.ManagedNodeGroupStatus = fmt.Sprintf("failed with error %v", v.Error)
		} else {
			ts.cfg.Status.ManagedNodeGroupStatus = ManagedNodeGroupStatusDELETEDORNOTEXIST
		}
		return
	}
	ts.cfg.Status.ManagedNodeGroupStatus = aws.StringValue(v.NodeGroup.Status)
	ts.cfg.Sync()
}

// ManangedNodeGroupDelete returns true if error from EKS API indicates that
// the EKS managed node group has already been deleted.
func ManangedNodeGroupDelete(err error) bool {
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

func (ts *Tester) waitForNodes() error {
	waitDur := 2*time.Minute + time.Duration(15*ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity)*time.Second

	ts.lg.Info("checking nodes via ASG")
	dout, err := ts.eksAPI.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   aws.String(ts.cfg.Name),
		NodegroupName: aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
	})
	if err != nil {
		return err
	}

	ts.cfg.Status.ManagedNodeGroupRemoteAccessSecurityGroupID = aws.StringValue(dout.Nodegroup.Resources.RemoteAccessSecurityGroup)
	ts.cfg.Sync()

	if ts.cfg.Status.ManagedNodeGroups == nil {
		ts.cfg.Status.ManagedNodeGroups = make(map[string]eksconfig.NodeGroup)
	}
	for _, asg := range dout.Nodegroup.Resources.AutoScalingGroups {
		asgName := aws.StringValue(asg.Name)

		var aout *autoscaling.DescribeAutoScalingGroupsOutput
		aout, err = ts.asgAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
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

		ts.lg.Info(
			"describing EC2 instances in ASG",
			zap.String("asg-name", asgName),
			zap.Strings("instance-ids", instanceIDs),
		)
		ec2Instances, err := awsapiec2.PollUntilRunning(
			waitDur,
			ts.lg,
			ts.ec2API,
			instanceIDs...,
		)
		if err != nil {
			return err
		}
		ng := eksconfig.NodeGroup{Instances: make(map[string]ec2config.Instance)}
		for id, vv := range ec2Instances {
			ng.Instances[id] = internalec2.ConvertEC2Instance(vv)
		}
		ts.cfg.Status.ManagedNodeGroups[asgName] = ng
		ts.cfg.Sync()
	}

	var items []v1.Node
	ts.lg.Info("checking nodes via client-go")
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitDur {
		nodes, err := ts.getAllNodes()
		if err != nil {
			ts.lg.Error("get nodes failed", zap.Error(err))
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
				ts.lg.Info("node info",
					zap.String("name", node.GetName()),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				if cond.Status == v1.ConditionTrue {
					readies++
				}
			}
		}
		ts.lg.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity),
		)
		if readies >= ts.cfg.Parameters.ManagedNodeGroupASGDesiredCapacity {
			break
		}
	}
	println()
	for _, v := range items {
		fmt.Printf("'Node' %q (using client-go): %+v\n", v.GetName(), v.Status.Addresses)
	}
	println()

	return ts.cfg.Sync()
}

func (ts *Tester) deleteManagedNodeGroup() error {
	if ts.cfg.Status.ManagedNodeGroupStatus == "" || ts.cfg.Status.ManagedNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
		ts.lg.Info("managed node group already deleted; no need to delete managed node group")
		return nil
	}

	ts.lg.Info("deleting managed node group", zap.String("managed-node-group-name", ts.cfg.Parameters.ManagedNodeGroupName))
	if ts.cfg.Status.ManagedNodeGroupCFNStackID != "" {
		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(ts.cfg.Status.ManagedNodeGroupCFNStackID),
		})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := awsapicfn.Poll(
			ctx,
			make(chan struct{}),  // do not exit on stop
			make(chan os.Signal), // do not exit on stop
			ts.lg,
			ts.cfnAPI,
			ts.cfg.Status.ManagedNodeGroupCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			2*time.Minute,
			15*time.Second,
		)
		var st awsapicfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.Status.ManagedNodeGroupStatus = fmt.Sprintf("failed to delete a managed node group (%v)", st.Error)
				ts.cfg.Sync()
				ts.lg.Error("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.Status.ManagedNodeGroupStatus = ManagedNodeGroupStatusDELETEDORNOTEXIST
		ts.cfg.Sync()

	} else {

		_, err := ts.eksAPI.DeleteNodegroup(&awseks.DeleteNodegroupInput{
			ClusterName:   aws.String(ts.cfg.Name),
			NodegroupName: aws.String(ts.cfg.Parameters.ManagedNodeGroupName),
		})
		if err != nil {
			ts.cfg.Status.ManagedNodeGroupStatus = fmt.Sprintf("failed to delete managed node group (%v)", err)
			ts.cfg.Sync()
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := PollEKSManagedNodeGroup(
			ctx,
			ts.stopCreationCh,
			ts.lg,
			ts.eksAPI,
			ts.cfg.Name,
			ts.cfg.Parameters.ManagedNodeGroupName,
			ManagedNodeGroupStatusDELETEDORNOTEXIST,
			3*time.Minute,
			20*time.Second,
		)
		for v := range ch {
			ts.updateManagedNodeGroupStatus(v)
		}
		cancel()
	}

	ts.lg.Info("deleted a managed node group",
		zap.String("managed-node-group-cfn-stack-id", ts.cfg.Status.ManagedNodeGroupCFNStackID),
		zap.String("managed-node-group-name", ts.cfg.Parameters.ManagedNodeGroupName),
	)
	return ts.cfg.Sync()
}
