package mng

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	awsapiec2 "github.com/aws/aws-k8s-tester/pkg/aws/ec2"
	"github.com/aws/aws-k8s-tester/version"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
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
        Name: !Ref Name
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
	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.CreateTookString = ts.cfg.EKSConfig.AddOnManagedNodeGroups.CreateTook.String()
	}()

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
			createInput := awseks.CreateNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(cur.Name),
				NodeRole:      aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleARN),
				AmiType:       aws.String(cur.AMIType),
				DiskSize:      aws.Int64(int64(cur.VolumeSize)),
				InstanceTypes: aws.StringSlice(cur.InstanceTypes),
				RemoteAccess: &awseks.RemoteAccessConfig{
					Ec2SshKey: aws.String(ts.cfg.EKSConfig.RemoteAccessKeyName),
				},
				ScalingConfig: &awseks.NodegroupScalingConfig{
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
					"Name": aws.String(cur.Name),
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

			cur.CreateRequested = true
			cur.Status = awseks.NodegroupStatusCreating
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
				Tags: awscfn.NewTags(map[string]string{
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

			stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
			if err != nil {
				return err
			}

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
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			ch := awscfn.Poll(
				ctx,
				ts.cfg.Stopc,
				ts.cfg.Sig,
				ts.cfg.Logger,
				ts.cfg.CFNAPI,
				mngStackID,
				cloudformation.ResourceStatusCreateComplete,
				2*time.Minute,
				15*time.Second,
			)
			var st awscfn.StackStatus
			for st = range ch {
				if st.Error != nil {
					cur.Status = fmt.Sprintf("failed to create managed node group (%v)", st.Error)
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
					ts.cfg.EKSConfig.Sync()
				}
			}
			cancel()
			for _, o := range st.Stack.Outputs {
				switch k := aws.StringValue(o.OutputKey); k {
				case "MNGID":
					cur.PhysicalID = aws.StringValue(o.OutputValue)
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
					ts.cfg.EKSConfig.Sync()
				default:
					ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("unexpected OutputKey %q from MNG stack %q", k, mngStackID))
					return fmt.Errorf("unexpected OutputKey %q from MNG stack %q", k, mngStackID)
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		ch := Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			mngName,
			awseks.NodegroupStatusActive,
			time.Minute,
			20*time.Second,
		)
		for sv := range ch {
			if serr := ts.setStatus(sv); serr != nil {
				cancel()
				return serr
			}
			ss := aws.StringValue(sv.NodeGroup.Status)
			if sv.Error != nil && ss == awseks.NodegroupStatusCreateFailed {
				ts.cfg.Logger.Warn("node group failed to create",
					zap.String("node-group-status", ss),
					zap.Error(sv.Error),
				)
				cancel()
				return sv.Error
			}
		}
		cancel()

		if err := ts.waitForNodes(cur.Name); err != nil {
			return err
		}

		ts.cfg.Logger.Info("created a managed node group",
			zap.String("mng-name", cur.Name),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteASG() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.DeleteTookString = ts.cfg.EKSConfig.AddOnManagedNodeGroups.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting managed node groups")
	for mngName, mv := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if mngName == "" {
			ts.cfg.Logger.Warn("empty name found in status map")
			delete(ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs, "")
			continue
		}
		if mv.Status == "" || mv.Status == ManagedNodeGroupStatusDELETEDORNOTEXIST {
			ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", mngName))
			continue
		}

		useCFN := mv.CFNStackID != ""
		if ts.failedOnce {
			useCFN = false
		}

		var err error
		if useCFN {
			ts.cfg.Logger.Info("deleting managed node group using CFN",
				zap.String("mng-name", mngName),
				zap.String("cfn-stack-id", mv.CFNStackID),
			)
			_, err = ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
				StackName: aws.String(mv.CFNStackID),
			})
		} else {
			ts.cfg.Logger.Info("deleting managed node group using EKS API", zap.String("name", mngName))
			_, err = ts.cfg.EKSAPI.DeleteNodegroup(&awseks.DeleteNodegroupInput{
				ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
				NodegroupName: aws.String(mngName),
			})
		}
		if err != nil {
			mv.Status = fmt.Sprintf("failed to delete managed node group (%v)", err)
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = mv
			ts.cfg.EKSConfig.Sync()
			return err
		}
	}

	for mngName, mv := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		if mv.Status == "" || mv.Status == ManagedNodeGroupStatusDELETEDORNOTEXIST {
			ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", mngName))
			continue
		}

		useCFN := mv.CFNStackID != ""
		if ts.failedOnce {
			useCFN = false
		}

		if useCFN {
			ts.cfg.Logger.Info("waiting for delete managed node group using CFN",
				zap.String("mng-name", mngName),
				zap.String("cfn-stack-id", mv.CFNStackID),
			)
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
					ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = mv
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

		mv.Status = ManagedNodeGroupStatusDELETEDORNOTEXIST
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = mv
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

			output, err := eksAPI.DescribeNodegroup(&awseks.DescribeNodegroupInput{
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
				lg.Info("became desired managed node group status; exiting", zap.String("status", currentStatus))
				close(ch)
				return
			case awseks.NodegroupStatusCreateFailed,
				awseks.NodegroupStatusDeleteFailed,
				awseks.NodegroupStatusDegraded:
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
	waitDur := 2*time.Minute + time.Duration(15*cur.ASGDesiredCapacity)*time.Second

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

	ts.cfg.Logger.Info(
		"describing EC2 instances in ASG",
		zap.String("asg-name", cur.ASGName),
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
	var items []v1.Node
	retryStart := time.Now()
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-ts.cfg.Sig:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items = nodes.Items

		readies := 0
		for _, node := range items {
			nodeName := node.GetName()

			// e.g. given node name ip-192-168-81-186.us-west-2.compute.internal + DHCP option my-private-dns
			// InternalIP == 192.168.81.186
			// ExternalIP == 52.38.118.149
			// Hostname == my-private-dns (without DHCP option, it's "ip-192-168-81-186.my-private-dns", private DNS, InternalDNS)
			// InternalDNS == ip-192-168-81-186.my-private-dns
			// ExternalDNS == ec2-52-38-118-149.us-west-2.compute.amazonaws.com
			ts.cfg.Logger.Info("checking node address with EC2 Private DNS",
				zap.String("name", nodeName),
				zap.String("labels", fmt.Sprintf("%v", node.Labels)),
			)
			hostName := ""
			for _, av := range node.Status.Addresses {
				ts.cfg.Logger.Info("node status address",
					zap.String("name", nodeName),
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
				ts.cfg.Logger.Warn("node may not belong to this ASG", zap.String("host-name", hostName), zap.String("ec2-private-dnss", fmt.Sprintf("%v", ec2PrivateDNS)))
				continue
			}
			ts.cfg.Logger.Info("checked node host name with EC2 Private DNS", zap.String("name", nodeName), zap.String("host-name", hostName))
			ts.cfg.Logger.Info("checking node readiness", zap.String("name", nodeName))
			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("checked node readiness",
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

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"get",
			"csr",
			"-o=wide",
		).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get csr' failed", zap.Error(err))
		}
		fmt.Printf("\n\n\"%s get csr\":\n%s\n", ts.cfg.EKSConfig.KubectlCommand(), out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"get",
			"nodes",
			"-o=wide",
		).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get nodes -o=wide' failed", zap.Error(err))
		}
		fmt.Printf("\n\"%s get nodes -o=wide\":\n%s\n\n", ts.cfg.EKSConfig.KubectlCommand(), out)

		if readies >= cur.ASGDesiredCapacity { // TODO: check per node group
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("MNG %q not ready", mngName)
	}

	println()
	fmt.Printf("%q nodes are ready!\n", mngName)
	for _, v := range items {
		fmt.Printf("node %q address: %+v\n", v.GetName(), v.Status.Addresses)
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
