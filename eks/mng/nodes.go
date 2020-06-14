package mng

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
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

func (ts *tester) createASGs() error {
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
			if cur.CreateRequested || cur.MNGCFNStackID != "" {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("mng-name", mngName),
					zap.Bool("create-requested", cur.CreateRequested),
					zap.String("cfn-stack-id", cur.MNGCFNStackID),
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
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created = true
			ts.cfg.EKSConfig.Sync()
			ts.cfg.Logger.Info("sent create managed node group request")

			tss = append(tss, tupleTime{ts: time.Now(), name: mngName})
			// when used with EKS API directly, just use "Poll" below to sync status
		}

	} else {

		for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			if cur.CreateRequested || cur.MNGCFNStackID != "" {
				ts.cfg.Logger.Warn("no need to create a new one, skipping",
					zap.String("mng-name", mngName),
					zap.Bool("create-requested", cur.CreateRequested),
					zap.String("cfn-stack-id", cur.MNGCFNStackID),
				)
				continue
			}

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

			if err := ioutil.WriteFile(cur.MNGCFNStackYAMLFilePath, buf.Bytes(), 0400); err != nil {
				return err
			}
			ts.cfg.Logger.Info("creating a new MNG using CFN",
				zap.String("mng-name", mngName),
				zap.String("mng-cfn-file-path", cur.MNGCFNStackYAMLFilePath),
			)

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
			cur.MNGCFNStackID = aws.StringValue(stackOutput.StackId)
			cur.Status = cloudformation.ResourceStatusCreateInProgress
			cur.Instances = make(map[string]ec2config.Instance)
			cur.Logs = make(map[string][]string)
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.Created = true
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
			return fmt.Errorf("MNGs[%q] not found after creation", mngName)
		}

		mngStackID := cur.MNGCFNStackID
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
					cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
					if !ok {
						cancel()
						return fmt.Errorf("MNGs[%q] not found after creation", mngName)
					}
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
					cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
					if !ok {
						return fmt.Errorf("MNGs[%q] not found after creation", mngName)
					}
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
		ch := wait.Poll(
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
			ss, serr := ts.setStatus(sv)
			if serr != nil {
				cancel()
				return serr
			}
			if sv.Error != nil && ss == aws_eks.NodegroupStatusCreateFailed {
				ts.cfg.Logger.Warn("failed to create managed node group",
					zap.String("status", ss),
					zap.Error(sv.Error),
				)
				cancel()
				return sv.Error
			}
		}
		cancel()
		cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		if !ok {
			return fmt.Errorf("MNGs[%q] not found after creation", mngName)
		}
		timeEnd := time.Now()
		cur.TimeFrameCreate = timeutil.NewTimeFrame(cur.TimeFrameCreate.StartUTC, cur.TimeFrameCreate.EndUTC.Add(timeEnd.Sub(timeStart)))
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()

		timeStart = time.Now()
		if err := ts.nodeWaiter.Wait(cur.Name, 3); err != nil {
			return err
		}
		cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		if !ok {
			return fmt.Errorf("MNGs[%q] not found after creation", mngName)
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

func (ts *tester) deleteASGs(mngName string) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot create ingress/egress security group", mngName)
	}
	if cur.Status == "" || cur.Status == wait.ManagedNodeGroupStatusDELETEDORNOTEXIST {
		ts.cfg.Logger.Info("managed node group already deleted; no need to delete managed node group", zap.String("name", mngName))
		return nil
	}

	ts.cfg.Logger.Info("deleting managed node group", zap.String("name", mngName))
	useCFN := cur.MNGCFNStackID != ""
	if _, ok := ts.deleteRequested[mngName]; ok {
		useCFN = false
	}

	if useCFN {
		ts.cfg.Logger.Info("deleting managed node group using CFN",
			zap.String("mng-name", mngName),
			zap.String("cfn-stack-id", cur.MNGCFNStackID),
		)
		_, err = ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(cur.MNGCFNStackID),
		})
	} else {
		ts.cfg.Logger.Info("deleting managed node group using EKS API", zap.String("name", mngName))
		_, err = ts.cfg.EKSAPI.DeleteNodegroup(&aws_eks.DeleteNodegroupInput{
			ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
			NodegroupName: aws.String(mngName),
		})
	}
	ts.deleteRequested[mngName] = struct{}{}

	if err != nil {
		if strings.Contains(err.Error(), "No cluster found for") {
			err = nil
			ts.cfg.Logger.Warn("deleted managed node group; cluster has already been deleted", zap.Error(err))
			cur.Status = fmt.Sprintf("deleted managed node group (%v)", err)
		} else {
			ts.cfg.Logger.Warn("failed to delete managed node group", zap.Error(err))
			cur.Status = fmt.Sprintf("failed to delete managed node group (%v)", err)
		}
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
		if err != nil {
			return err
		}
	}

	timeStart := time.Now()

	if useCFN {
		ts.cfg.Logger.Info("waiting for delete managed node group using CFN",
			zap.String("mng-name", mngName),
			zap.String("cfn-stack-id", cur.MNGCFNStackID),
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
			cur.MNGCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			initialWait,
			15*time.Second,
		)
		var st cfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				var ok bool
				cur, ok = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
				if !ok {
					cancel()
					return fmt.Errorf("MNGs[%q] not found after creation", mngName)
				}
				timeEnd := time.Now()
				cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
				if strings.Contains(st.Error.Error(), "No cluster found for") {
					st.Error = nil
					ts.cfg.Logger.Warn("deleted managed node group; cluster has already been deleted", zap.Error(st.Error))
					cur.Status = fmt.Sprintf("deleted a managed node group (%v)", st.Error)
				} else {
					ts.cfg.Logger.Warn("failed to delete managed node group", zap.Error(st.Error))
					cur.Status = fmt.Sprintf("failed to delete a managed node group (%v)", st.Error)
				}
				ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
				ts.cfg.EKSConfig.Sync()
				if st.Error == nil {
					break
				}
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
		ch := wait.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			mngName,
			wait.ManagedNodeGroupStatusDELETEDORNOTEXIST,
			initialWait,
			20*time.Second,
		)
		for sv := range ch {
			ss, serr := ts.setStatus(sv)
			if serr != nil {
				cancel()
				return serr
			}
			if sv.Error != nil {
				if ss == aws_eks.NodegroupStatusDeleteFailed {
					ts.cfg.Logger.Warn("failed to delete managed node group",
						zap.String("status", ss),
						zap.Error(sv.Error),
					)
					cancel()
					return sv.Error
				}
				if strings.Contains(sv.Error.Error(), "No cluster found for") {
					sv.Error = nil
					ts.cfg.Logger.Warn("deleted managed node group; cluster has already been deleted",
						zap.String("status", ss),
						zap.Error(sv.Error),
					)
					break
				}
				ts.cfg.Logger.Warn("failed to delete managed node group",
					zap.String("status", ss),
					zap.Error(sv.Error),
				)
			}
		}
		cancel()
	}

	timeEnd := time.Now()
	cur.TimeFrameDelete = timeutil.NewTimeFrame(timeStart, timeEnd)
	cur.Status = wait.ManagedNodeGroupStatusDELETEDORNOTEXIST
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("deleted managed node group", zap.String("name", mngName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) setStatus(sv wait.ManagedNodeGroupStatus) (status string, err error) {
	name := sv.NodeGroupName
	if name == "" {
		return "", errors.New("EKS Managed Node Group empty name")
	}
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return "", fmt.Errorf("EKS MNGs[%q] not found", name)
	}

	if sv.NodeGroup == nil {
		if sv.Error != nil {
			cur.Status = fmt.Sprintf("%q failed with error %v", sv.NodeGroupName, sv.Error)
		} else {
			cur.Status = wait.ManagedNodeGroupStatusDELETEDORNOTEXIST
		}
	} else {
		cur.Status = aws.StringValue(sv.NodeGroup.Status)
		if sv.NodeGroup.Resources != nil && cur.RemoteAccessSecurityGroupID == "" {
			cur.RemoteAccessSecurityGroupID = aws.StringValue(sv.NodeGroup.Resources.RemoteAccessSecurityGroup)
		}
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name] = cur
	return cur.Status, ts.cfg.EKSConfig.Sync()
}
