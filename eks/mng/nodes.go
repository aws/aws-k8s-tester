package mng

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_ec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	k8s_object "github.com/aws/aws-k8s-tester/pkg/k8s-object"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
)

// TemplateMNG is the CloudFormation template for EKS managed node group.
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
const TemplateMNG = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Managed Node Group'

Parameters:

  ClusterName:
    Type: String
    Description: The cluster name provided when the cluster was created. If it is incorrect, nodes will not be able to join the cluster.

  Name:
    Type: String
    Description: Unique identifier for the Node Group.

  RoleARN:
    Type: String
    Description: The ARN of the node instance role

  PublicSubnetIDs:
    Type: List<AWS::EC2::Subnet::Id>
    Description: The public subnet IDs where workers can be created.

  RemoteAccessKeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: Amazon EC2 Key Pair

  AMIType:
    Type: String
    Default: AL2_x86_64
    AllowedValues:
    - AL2_x86_64
    - AL2_x86_64_GPU
    Description: AMI type for your node group

  ASGMinSize:
    Type: Number
    Default: 2
    Description: Minimum size of Node Group Auto Scaling Group.

  ASGMaxSize:
    Type: Number
    Default: 2
    Description: Maximum size of Node Group Auto Scaling Group. Set to at least 1 greater than ASGDesiredCapacity.

  ASGDesiredCapacity:
    Type: Number
    Default: 2
    Description: Desired capacity of Node Group Auto Scaling Group.

  InstanceTypes:
    Type: CommaDelimitedList
    Default: c5.xlarge
    Description: EC2 instance types for the node instances

  VolumeSize:
    Type: Number
    Default: 40
    Description: Node volume size

{{ if ne .ParameterReleaseVersion "" }}{{.ParameterReleaseVersion}}{{ end }}

Resources:

  MNG:
    Type: AWS::EKS::Nodegroup
    Properties:
      ClusterName: !Ref ClusterName
      NodegroupName: !Ref Name
      NodeRole: !Ref RoleARN
      AmiType: !Ref AMIType
      DiskSize: !Ref VolumeSize
      InstanceTypes: !Ref InstanceTypes
      RemoteAccess:
        Ec2SshKey: !Ref RemoteAccessKeyName
      ScalingConfig:
        MinSize: !Ref ASGMinSize
        MaxSize: !Ref ASGMaxSize
        DesiredSize: !Ref ASGDesiredCapacity
      Subnets: !Ref PublicSubnetIDs
      Labels:
        NodeType: regular
        AMIType: !Ref AMIType
        NGType: managed
        NGName: !Ref Name
{{ if ne .PropertyReleaseVersion "" }}{{.PropertyReleaseVersion}}{{ end }}

Outputs:

  MNGID:
    Value: !Ref MNG
    Description: The managed node group resource Physical ID

`

const parametersReleaseVersion = `  ReleaseVersion:
    Type: String
    Description: AMI version of the Amazon EKS-optimized AMI`

const propertyReleaseVersion = `      ReleaseVersion: !Ref ReleaseVersion`

type templateMNG struct {
	ParameterReleaseVersion string
	PropertyReleaseVersion  string
}

func (ts *tester) createASG() error {
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN == "" {
		return errors.New("empty AddOnManagedNodeGroups.RoleARN")
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

	// track timestamps and check status in reverse order
	// to minimize polling API calls
	tss := make(tupleTimes, 0)

	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.ResolverURL != "" ||
		(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderKey != "" &&
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.RequestHeaderValue != "") {

		for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			vv, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
			if ok && (vv.CreateRequested || vv.CFNStackID != "") {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("mng-name", mngName),
					zap.Bool("create-requested", vv.CreateRequested),
					zap.String("cfn-stack-id", vv.CFNStackID),
				)
				continue
			}

			ts.cfg.Logger.Info("creating a managed node group using EKS API", zap.String("mng-name", cur.Name))
			createInput := aws_eks.CreateNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(cur.Name),
				NodeRole:      aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
				AmiType:       aws.String(cur.AMIType),
				DiskSize:      aws.Int64(int64(cur.VolumeSize)),
				InstanceTypes: aws.StringSlice(cur.InstanceTypes),
				RemoteAccess: &aws_eks.RemoteAccessConfig{
					Ec2SshKey: aws.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
				},
				ScalingConfig: &aws_eks.NodegroupScalingConfig{
					MinSize:     aws.Int64(int64(cur.ASGMinSize)),
					MaxSize:     aws.Int64(int64(cur.ASGMaxSize)),
					DesiredSize: aws.Int64(int64(cur.ASGDesiredCapacity)),
				},
				Subnets: aws.StringSlice(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs),
				Tags: map[string]*string{
					"Kind":                   aws.String("aws-k8s-tester"),
					"aws-k8s-tester-version": aws.String(version.ReleaseVersion),
				},
				Labels: map[string]*string{
					"NodeType": aws.String("normal"),
					"AMIType":  aws.String(cur.AMIType),
					"NGType":   aws.String("managed"),
					"NGName":   aws.String(cur.Name),
				},
			}
			for k, v := range cur.Tags {
				createInput.Tags[k] = aws.String(v)
				ts.cfg.Logger.Info("added EKS tag", zap.String("key", k), zap.String("value", v))
			}
			if cur.ReleaseVersion != "" {
				createInput.ReleaseVersion = aws.String(cur.ReleaseVersion)
				ts.cfg.Logger.Info("added EKS release version", zap.String("version", cur.ReleaseVersion))
			}

			timeStart := time.Now()
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

			cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, time.Now())
			cur.CreateRequested = true
			cur.Status = aws_eks.NodegroupStatusCreating
			cur.Instances = make(map[string]ec2config.Instance)
			cur.Logs = make(map[string][]string)
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
			ts.cfg.EKSConfig.Sync()
			ts.cfg.Logger.Info("sent create managed node group request")

			tss = append(tss, tupleTime{ts: time.Now(), name: mngName})
			// when used with EKS API directly, just use "Poll" below to sync status
		}

	} else {

		for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			vv, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
			if ok && (vv.CreateRequested || vv.CFNStackID != "") {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("mng-name", mngName),
					zap.Bool("create-requested", vv.CreateRequested),
					zap.String("cfn-stack-id", vv.CFNStackID),
				)
				continue
			}

			ts.cfg.Logger.Info("creating a new node group using CFN", zap.String("mng-name", cur.Name))
			stackInput := &cloudformation.CreateStackInput{
				StackName:    aws.String(cur.Name),
				Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
				OnFailure:    aws.String(cloudformation.OnFailureDelete),
				Tags: cfn.NewTags(map[string]string{
					"Kind":                   "aws-k8s-tester",
					"Name":                   ts.cfg.EKSConfig.Name,
					"aws-k8s-tester-version": version.ReleaseVersion,
				}),
				Parameters: []*cloudformation.Parameter{
					{
						ParameterKey:   aws.String("ClusterName"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.Name),
					},
					{
						ParameterKey:   aws.String("Name"),
						ParameterValue: aws.String(cur.Name),
					},
					{
						ParameterKey:   aws.String("RoleARN"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
					},
					{
						ParameterKey:   aws.String("PublicSubnetIDs"),
						ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs, ",")),
					},
					{
						ParameterKey:   aws.String("RemoteAccessKeyName"),
						ParameterValue: aws.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
					},
				},
			}

			tg := templateMNG{}
			if cur.ReleaseVersion != "" {
				tg.ParameterReleaseVersion = parametersReleaseVersion
				tg.PropertyReleaseVersion = propertyReleaseVersion
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ReleaseVersion"),
					ParameterValue: aws.String(cur.ReleaseVersion),
				})
			}
			tpl := template.Must(template.New("TemplateMNG").Parse(TemplateMNG))
			buf := bytes.NewBuffer(nil)
			if err := tpl.Execute(buf, tg); err != nil {
				return err
			}
			stackInput.TemplateBody = aws.String(buf.String())

			if cur.AMIType != "" {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("AMIType"),
					ParameterValue: aws.String(cur.AMIType),
				})
			}
			if cur.ASGMinSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ASGMinSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGMinSize)),
				})
			}
			if cur.ASGMaxSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ASGMaxSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGMaxSize)),
				})
			}
			if cur.ASGDesiredCapacity > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("ASGDesiredCapacity"),
					ParameterValue: aws.String(fmt.Sprintf("%d", cur.ASGDesiredCapacity)),
				})
			}
			if len(cur.InstanceTypes) > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("InstanceTypes"),
					ParameterValue: aws.String(strings.Join(cur.InstanceTypes, ",")),
				})
			}
			if cur.VolumeSize > 0 {
				stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
					ParameterKey:   aws.String("VolumeSize"),
					ParameterValue: aws.String(fmt.Sprintf("%d", cur.VolumeSize)),
				})
			}

			timeStart := time.Now()
			stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
			if err != nil {
				return err
			}

			cur.TimeFrameCreate = timeutil.NewTimeFrame(timeStart, time.Now())
			cur.CreateRequested = true
			cur.CFNStackID = aws.StringValue(stackOutput.StackId)
			cur.Status = cloudformation.ResourceStatusCreateInProgress
			cur.Instances = make(map[string]ec2config.Instance)
			cur.Logs = make(map[string][]string)
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
			ts.cfg.EKSConfig.Sync()

			tss = append(tss, tupleTime{ts: time.Now(), name: mngName})
		}
	}

	sort.Sort(sort.Reverse(tss))

	// wait for ASG EC2 instances + MNG nodes + Kubernetes nodes ready
	for _, tv := range tss {
		mngName := tv.name
		cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		if !ok {
			return fmt.Errorf("MNG name %q not found after creation", mngName)
		}

		mngStackID := cur.CFNStackID
		if mngStackID != "" {
			timeStart := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			ch := cfn.Poll(
				ctx,
				ts.cfg.Stopc,
				ts.cfg.Logger,
				ts.cfg.CFNAPI,
				mngStackID,
				cloudformation.ResourceStatusCreateComplete,
				2*time.Minute,
				15*time.Second,
			)
			var st cfn.StackStatus
			for st = range ch {
				if st.Error != nil {
					timeEnd := time.Now()
					cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
					cur.Status = fmt.Sprintf("failed to create managed node group (%v)", st.Error)
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
					ts.cfg.EKSConfig.Sync()
				}
			}
			cancel()
			for _, o := range st.Stack.Outputs {
				switch k := aws.StringValue(o.OutputKey); k {
				case "MNGID":
					timeEnd := time.Now()
					cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
					cur.PhysicalID = aws.StringValue(o.OutputValue)
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
					ts.cfg.EKSConfig.Sync()
				default:
					ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("unexpected OutputKey %q from MNG stack %q", k, mngStackID))
					return fmt.Errorf("unexpected OutputKey %q from MNG stack %q", k, mngStackID)
				}
			}
		}

		timeStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			mngName,
			aws_eks.NodegroupStatusActive,
			time.Minute,
			20*time.Second,
		)
		for sv := range ch {
			if serr := ts.setStatus(sv); serr != nil {
				cancel()
				return serr
			}
			ss := aws.StringValue(sv.NodeGroup.Status)
			if sv.Error != nil && ss == aws_eks.NodegroupStatusCreateFailed {
				ts.cfg.Logger.Warn("node group failed to create",
					zap.String("node-group-status", ss),
					zap.Error(sv.Error),
				)
				cancel()
				return sv.Error
			}
		}
		cancel()
		timeEnd := time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()

		timeStart = time.Now()
		if err := ts.waitForNodes(cur.Name); err != nil {
			return err
		}
		timeEnd = time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()

		ts.cfg.Logger.Info("created a managed node group",
			zap.String("mng-name", cur.Name),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteASG() error {
	ts.cfg.Logger.Info("deleting managed node groups")
	for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if mngName == "" {
			ts.cfg.Logger.Warn("empty name found in status map")
			delete(ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs, "")
			continue
		}
		if cur.Status == "" || cur.Status == ManagedNodeGroupStatusDELETEDORNOTEXIST {
			ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", mngName))
			continue
		}

		useCFN := cur.CFNStackID != ""
		if ts.failedOnce {
			useCFN = false
		}

		var err error
		if useCFN {
			ts.cfg.Logger.Info("deleting managed node group using CFN",
				zap.String("mng-name", mngName),
				zap.String("cfn-stack-id", cur.CFNStackID),
			)
			_, err = ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: aws.String(cur.CFNStackID),
			})
		} else {
			ts.cfg.Logger.Info("deleting managed node group using EKS API", zap.String("name", mngName))
			_, err = ts.cfg.EKSAPI.DeleteNodegroup(&aws_eks.DeleteNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(mngName),
			})
		}
		if err != nil {
			cur.Status = fmt.Sprintf("failed to delete managed node group (%v)", err)
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
			ts.cfg.EKSConfig.Sync()
			return err
		}
	}

	for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if cur.Status == "" || cur.Status == ManagedNodeGroupStatusDELETEDORNOTEXIST {
			ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", mngName))
			continue
		}

		useCFN := cur.CFNStackID != ""
		if ts.failedOnce {
			useCFN = false
		}

		timeStart := time.Now()

		if useCFN {
			ts.cfg.Logger.Info("waiting for delete managed node group using CFN",
				zap.String("mng-name", mngName),
				zap.String("cfn-stack-id", cur.CFNStackID),
			)
			initialWait, timeout := 2*time.Minute, 15*time.Minute
			if len(cur.Instances) > 50 {
				initialWait, timeout = 3*time.Minute, 20*time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ch := cfn.Poll(
				ctx,
				make(chan struct{}), // do not exit on stop
				ts.cfg.Logger,
				ts.cfg.CFNAPI,
				cur.CFNStackID,
				cloudformation.ResourceStatusDeleteComplete,
				initialWait,
				15*time.Second,
			)
			var st cfn.StackStatus
			for st = range ch {
				if st.Error != nil {
					cancel()
					timeEnd := time.Now()
					cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
					cur.Status = fmt.Sprintf("failed to delete a managed node group (%v)", st.Error)
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
					ts.cfg.EKSConfig.Sync()
					ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
				}
			}
			cancel()
			if st.Error != nil {
				return st.Error
			}

		} else {

			ts.cfg.Logger.Info("waiting for delete managed node group using EKS API", zap.String("name", mngName))
			initialWait, timeout := 2*time.Minute, 15*time.Minute
			if len(cur.Instances) > 50 {
				initialWait, timeout = 3*time.Minute, 20*time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			ch := Poll(
				ctx,
				ts.cfg.Stopc,
				ts.cfg.Logger,
				ts.cfg.EKSAPI,
				ts.cfg.EKSConfig.Name,
				mngName,
				ManagedNodeGroupStatusDELETEDORNOTEXIST,
				initialWait,
				20*time.Second,
			)
			for v := range ch {
				if serr := ts.setStatus(v); serr != nil {
					cancel()
					return serr
				}
			}
			cancel()
		}

		timeEnd := time.Now()
		cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
		cur.Status = ManagedNodeGroupStatusDELETEDORNOTEXIST
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
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
	NodeGroup     *aws_eks.Nodegroup
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
	mngName string,
	desiredNodeGroupStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan ManagedNodeGroupStatus {
	lg.Info("polling mng",
		zap.String("cluster-name", clusterName),
		zap.String("mng-name", mngName),
		zap.String("desired-mng-status", desiredNodeGroupStatus),
	)

	now := time.Now()

	ch := make(chan ManagedNodeGroupStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		waitDur := time.Duration(0)

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = wait
				}
			}

			output, err := eksAPI.DescribeNodegroup(&aws_eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(mngName),
			})
			if err != nil {
				if IsDeleted(err) {
					if desiredNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
						lg.Info("managed node group is already deleted as desired; exiting", zap.Error(err))
						ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("managed node group does not exist", zap.Error(err))
					lg.Warn("aborting", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe managed node group failed; retrying", zap.Error(err))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
				continue
			}

			if output.Nodegroup == nil {
				lg.Warn("expected non-nil managed node group; retrying")
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			nodeGroup := output.Nodegroup
			currentStatus := aws.StringValue(nodeGroup.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("mng-name", mngName),
				zap.String("mng-status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredNodeGroupStatus:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
				lg.Info("desired managed node group status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case aws_eks.NodegroupStatusCreateFailed,
				aws_eks.NodegroupStatusDeleteFailed,
				aws_eks.NodegroupStatusDegraded:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: fmt.Errorf("unexpected mng status %q", currentStatus)}
				close(ch)
				return
			default:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
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
	mv, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
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
		if sv.NodeGroup.Resources != nil && mv.RemoteAccessSecurityGroupID == "" {
			mv.RemoteAccessSecurityGroupID = aws.StringValue(sv.NodeGroup.Resources.RemoteAccessSecurityGroup)
		}
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name] = mv
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitForNodes(mngName string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}

	ts.cfg.Logger.Info("checking MNG", zap.String("mng-name", cur.Name))
	dout, err := ts.cfg.EKSAPI.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(cur.Name),
	})
	if err != nil {
		return err
	}
	if dout.Nodegroup == nil {
		return fmt.Errorf("MNG %q not found", cur.Name)
	}
	if dout.Nodegroup.Resources == nil {
		return fmt.Errorf("MNG %q Resources not found", cur.Name)
	}
	if len(dout.Nodegroup.Resources.AutoScalingGroups) != 1 {
		return fmt.Errorf("expected 1 ASG for %q, got %d", mngName, len(dout.Nodegroup.Resources.AutoScalingGroups))
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		cur.RemoteAccessSecurityGroupID = aws.StringValue(dout.Nodegroup.Resources.RemoteAccessSecurityGroup)
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}
	asg := dout.Nodegroup.Resources.AutoScalingGroups[0]
	cur.ASGName = aws.StringValue(asg.Name)
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()
	ts.cfg.Logger.Info("checking MNG ASG", zap.String("mng-name", cur.Name), zap.String("asg-name", cur.ASGName))

	var aout *autoscaling.DescribeAutoScalingGroupsOutput
	aout, err = ts.cfg.ASGAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{cur.ASGName}),
	})
	if err != nil {
		return fmt.Errorf("ASG %q not found (%v)", cur.ASGName, err)
	}
	if len(aout.AutoScalingGroups) != 1 {
		return fmt.Errorf("%q expected only 1 ASG, got %+v", cur.ASGName, aout.AutoScalingGroups)
	}

	av := aout.AutoScalingGroups[0]
	instanceIDs := make([]string, 0, len(av.Instances))
	for _, iv := range av.Instances {
		instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
	}

	waitDur := 3*time.Minute + time.Duration(5*cur.ASGDesiredCapacity)*time.Second
	ts.cfg.Logger.Info(
		"describing EC2 instances in ASG",
		zap.String("asg-name", cur.ASGName),
		zap.Strings("instance-ids", instanceIDs),
		zap.Duration("wait", waitDur),
	)
	ec2Instances, err := aws_ec2.PollUntilRunning(
		waitDur,
		ts.cfg.Logger,
		ts.cfg.EC2API,
		instanceIDs...,
	)
	if err != nil {
		return err
	}
	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}
	cur.Instances = make(map[string]ec2config.Instance)
	for id, vv := range ec2Instances {
		ivv := ec2config.ConvertInstance(vv)
		ivv.RemoteAccessUserName = cur.RemoteAccessUserName
		cur.Instances[id] = ivv
	}
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("Managed Node Group %q not found", mngName)
	}

	// Hostname/InternalDNS == EC2 private DNS
	// TODO: handle DHCP option domain name
	ec2PrivateDNS := make(map[string]struct{})
	for _, v := range cur.Instances {
		ts.cfg.Logger.Info("found private DNS for an EC2 instance", zap.String("instance-id", v.InstanceID), zap.String("private-dns-name", v.PrivateDNSName))
		ec2PrivateDNS[v.PrivateDNSName] = struct{}{}
		// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
		ec2PrivateDNS[strings.Split(v.PrivateDNSName, ".")[0]] = struct{}{}
	}

	ts.cfg.Logger.Info("checking nodes readiness")
	retryStart := time.Now()
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		nodes, err := ts.cfg.K8SClient.ListNodes(150, 5*time.Second)
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}

		readies := 0
		for _, node := range nodes {
			labels := node.GetLabels()
			if labels["NGName"] != mngName {
				continue
			}
			nodeName := node.GetName()
			nodeInfo, _ := json.Marshal(k8s_object.ParseNodeInfo(node.Status.NodeInfo))

			// e.g. given node name ip-192-168-81-186.us-west-2.compute.internal + DHCP option my-private-dns
			// InternalIP == 192.168.81.186
			// ExternalIP == 52.38.118.149
			// Hostname == my-private-dns (without DHCP option, it's "ip-192-168-81-186.my-private-dns", private DNS, InternalDNS)
			// InternalDNS == ip-192-168-81-186.my-private-dns
			// ExternalDNS == ec2-52-38-118-149.us-west-2.compute.amazonaws.com
			ts.cfg.Logger.Info("checking node address with EC2 Private DNS",
				zap.String("node-name", nodeName),
				zap.String("node-info", string(nodeInfo)),
				zap.String("labels", fmt.Sprintf("%v", labels)),
			)

			hostName := ""
			for _, av := range node.Status.Addresses {
				ts.cfg.Logger.Info("node status address",
					zap.String("node-name", nodeName),
					zap.String("type", string(av.Type)),
					zap.String("address", string(av.Address)),
				)
				if av.Type != v1.NodeHostName && av.Type != v1.NodeInternalDNS {
					continue
				}
				// handle when node is configured DHCP
				hostName = av.Address
				_, ok := ec2PrivateDNS[hostName]
				if !ok {
					// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
					_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
				}
				if ok {
					break
				}
			}
			if hostName == "" {
				return fmt.Errorf("%q not found for node %q", v1.NodeHostName, nodeName)
			}
			_, ok := ec2PrivateDNS[hostName]
			if !ok {
				// "ip-192-168-81-186" from "ip-192-168-81-186.my-private-dns"
				_, ok = ec2PrivateDNS[strings.Split(hostName, ".")[0]]
			}
			if !ok {
				ts.cfg.Logger.Warn("node may not belong to this ASG", zap.String("host-name", hostName), zap.Int("ec2-private-dnss", len(ec2PrivateDNS)))
				continue
			}
			ts.cfg.Logger.Debug("checked node host name with EC2 Private DNS", zap.String("name", nodeName), zap.String("host-name", hostName))

			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("node is ready!",
					zap.String("name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				readies++
				break
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", cur.ASGDesiredCapacity),
		)

		/*
			e.g.
			"/tmp/kubectl-test-v1.16.9 --kubeconfig=/tmp/leegyuho-test-eks.kubeconfig.yaml get csr -o=wide":
			NAME        AGE   REQUESTOR                                                   CONDITION
			csr-4msk5   58s   system:node:ip-192-168-65-124.us-west-2.compute.internal    Approved,Issued
			csr-9dbs8   57s   system:node:ip-192-168-208-6.us-west-2.compute.internal     Approved,Issued
		*/
		output, err := ts.cfg.K8SClient.ListCSRs(150, 5*time.Second)
		if err != nil {
			ts.cfg.Logger.Warn("list CSRs failed", zap.Error(err))
		} else {
			for _, cv := range output {
				ts.cfg.Logger.Info("current CSR",
					zap.String("name", cv.GetName()),
					zap.String("requester", cv.Spec.Username),
					zap.String("status", extractCSRStatus(cv)),
				)
			}
		}

		if readies >= cur.ASGDesiredCapacity {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("MNG %q not ready", mngName)
	}

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

// "pkg/printers/internalversion/printers.go"
func extractCSRStatus(csr certificatesv1beta1.CertificateSigningRequest) string {
	var approved, denied bool
	for _, c := range csr.Status.Conditions {
		switch c.Type {
		case certificatesv1beta1.CertificateApproved:
			approved = true
		case certificatesv1beta1.CertificateDenied:
			denied = true
		default:
			return ""
		}
	}
	var status string
	// must be in order of presidence
	if denied {
		status += "Denied"
	} else if approved {
		status += "Approved"
	} else {
		status += "Pending"
	}
	if len(csr.Status.Certificate) > 0 {
		status += ",Issued"
	}
	return status
}
