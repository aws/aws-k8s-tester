package eks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	internalec2 "github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/httputil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

type workerNodeAMISSM struct {
	SchemaVersion string `json:"schema_version"`
	ImageID       string `json:"image_id"`
	ImageName     string `json:"image_name"`
}

// https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
// https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const workerNodeStackTemplateURL = "https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml"

func (md *embedded) createWorkerNode() error {
	if md.cfg.ClusterState.CFStackWorkerNodeGroupKeyPairName == "" {
		return errors.New("cannot create worker node without key name")
	}
	if md.cfg.ClusterState.CFStackWorkerNodeGroupName == "" {
		return errors.New("cannot create empty worker node")
	}

	if md.cfg.WorkerNodeAMIID == "" {
		// https://aws.amazon.com/about-aws/whats-new/2019/09/amazon-eks-provides-eks-optimized-ami-metadata-via-ssm-parameters/
		ssmKey := fmt.Sprintf("/aws/service/eks/optimized-ami/%s/%s/recommended", md.cfg.KubernetesVersion, md.cfg.WorkerNodeAMIType)
		md.lg.Info("getting SSM parameter to get latest worker node AMI", zap.String("ssm-key", ssmKey))
		so, err := md.ssm.GetParameters(&ssm.GetParametersInput{
			Names: aws.StringSlice([]string{ssmKey}),
		})
		if err != nil {
			return fmt.Errorf("failed to get latest worker node AMI %v", err)
		}
		value := ""
		for _, pm := range so.Parameters {
			if *pm.Name != ssmKey {
				continue
			}
			value = *pm.Value
		}
		if value == "" {
			return fmt.Errorf("SSM key %q not found", ssmKey)
		}
		var output workerNodeAMISSM
		if err = json.Unmarshal([]byte(value), &output); err != nil {
			return err
		}
		if output.ImageID == "" || output.ImageName == "" {
			return fmt.Errorf("latest worker node AMI not found (AMI %q, name %q)", output.ImageID, output.ImageName)
		}
		md.cfg.WorkerNodeAMIID = output.ImageID
		md.cfg.WorkerNodeAMIName = output.ImageName
		md.cfg.Sync()
		md.lg.Info("successfully got latest worker node AMI from SSM parameter",
			zap.String("worker-node-ami-type", md.cfg.WorkerNodeAMIType),
			zap.String("worker-node-ami-id", output.ImageID),
			zap.String("worker-node-ami-name", output.ImageName),
		)
	}

	var body []byte
	var err error
	if md.cfg.WorkerNodeCFTemplatePath == "" {
		body, err = httputil.Download(md.lg, os.Stdout, workerNodeStackTemplateURL)
	} else {
		md.lg.Info("reading worker node cloudformation template file", zap.String("path", md.cfg.WorkerNodeCFTemplatePath))
		body, err = ioutil.ReadFile(md.cfg.WorkerNodeCFTemplatePath)
	}
	if err != nil {
		return err
	}

	subnetIDs := md.cfg.SubnetIDs
	if !md.cfg.EnableWorkerNodeHA {
		subnetIDs = subnetIDs[:1]
		md.lg.Info("HA mode is disabled", zap.Strings("subnet-ids", subnetIDs))
	}
	params := []*cloudformation.Parameter{
		{
			ParameterKey:   aws.String("ClusterName"),
			ParameterValue: aws.String(md.cfg.ClusterName),
		},
		{
			ParameterKey:   aws.String("NodeGroupName"),
			ParameterValue: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		},
		{
			ParameterKey:   aws.String("KeyName"),
			ParameterValue: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupKeyPairName),
		},
		{
			ParameterKey:   aws.String("NodeImageId"),
			ParameterValue: aws.String(md.cfg.WorkerNodeAMIID),
		},
		{
			ParameterKey:   aws.String("NodeInstanceType"),
			ParameterValue: aws.String(md.cfg.WorkerNodeInstanceType),
		},
		{
			ParameterKey:   aws.String("NodeAutoScalingGroupMinSize"),
			ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkerNodeASGMin)),
		},
		{
			ParameterKey:   aws.String("NodeAutoScalingGroupMaxSize"),
			ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkerNodeASGMax)),
		},
		{
			ParameterKey:   aws.String("NodeAutoScalingGroupDesiredCapacity"),
			ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkerNodeASGDesiredCapacity)),
		},
		{
			ParameterKey:   aws.String("NodeVolumeSize"),
			ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkerNodeVolumeSizeGB)),
		},
		{
			ParameterKey:   aws.String("VpcId"),
			ParameterValue: aws.String(md.cfg.VPCID),
		},
		{
			ParameterKey:   aws.String("Subnets"),
			ParameterValue: aws.String(strings.Join(subnetIDs, ",")),
		},
		{
			ParameterKey:   aws.String("ClusterControlPlaneSecurityGroup"),
			ParameterValue: aws.String(md.cfg.SecurityGroupID),
		},
	}
	for _, k := range md.cfg.WorkerNodeCFTemplateAdditionalParameterKeys {
		switch k {
		case "CertificateAuthorityData":
			params = append(params, &cloudformation.Parameter{
				ParameterKey:   aws.String("CertificateAuthorityData"),
				ParameterValue: aws.String(md.cfg.ClusterState.CA),
			})
		case "ApiServerEndpoint":
			params = append(params, &cloudformation.Parameter{
				ParameterKey:   aws.String("ApiServerEndpoint"),
				ParameterValue: aws.String(md.cfg.ClusterState.Endpoint),
			})
		default:
			return fmt.Errorf("unknown worker node cloudformation parameter %q", k)
		}
	}

	now := time.Now().UTC()
	h, _ := os.Hostname()
	_, err = md.cfn.CreateStack(&cloudformation.CreateStackInput{
		StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		Tags: []*cloudformation.Tag{
			{Key: aws.String("Kind"), Value: aws.String("aws-k8s-tester")},
			{Key: aws.String("Creation"), Value: aws.String(time.Now().UTC().String())},
			{Key: aws.String("Name"), Value: aws.String(md.cfg.ClusterName)},
			{Key: aws.String("HOSTNAME"), Value: aws.String(h)},
		},

		TemplateBody: aws.String(string(body)),
		Parameters:   params,

		Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusWorkerNodeCreated = true
	md.cfg.Sync()

	// usually takes 3-minute
	md.lg.Info("waiting for 2-minute")
	select {
	case <-md.stopc:
		md.lg.Info("interrupted worker node creation")
		return nil
	case <-time.After(2 * time.Minute):
	}

	waitTime := 7*time.Minute + 2*time.Duration(md.cfg.WorkerNodeASGMax)*time.Minute
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		var do *cloudformation.DescribeStacksOutput
		do, err = md.cfn.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		})
		if err != nil {
			md.lg.Warn("failed to describe worker node", zap.Error(err))
			md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = err.Error()
			md.cfg.Sync()
			time.Sleep(20 * time.Second)
			continue
		}

		if len(do.Stacks) != 1 {
			return fmt.Errorf("%q expects 1 Stack, got %v", md.cfg.ClusterState.CFStackWorkerNodeGroupName, do.Stacks)
		}

		md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = *do.Stacks[0].StackStatus
		if isCFCreateFailed(md.cfg.ClusterState.CFStackWorkerNodeGroupStatus) {
			return fmt.Errorf("failed to create %q (%q)",
				md.cfg.ClusterState.CFStackWorkerNodeGroupName,
				md.cfg.ClusterState.CFStackWorkerNodeGroupStatus,
			)
		}
		md.lg.Info(
			"worker node cloud formation in progress",
			zap.String("stack-name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
			zap.String("stack-status", md.cfg.ClusterState.CFStackWorkerNodeGroupStatus),
		)
		if md.cfg.ClusterState.CFStackWorkerNodeGroupStatus != "CREATE_COMPLETE" {
			time.Sleep(20 * time.Second)
			continue
		}

		for _, op := range do.Stacks[0].Outputs {
			if *op.OutputKey == "NodeInstanceRole" {
				md.cfg.ClusterState.CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN = *op.OutputValue
			}
			if *op.OutputKey == "NodeSecurityGroup" { // not "SecurityGroups"
				md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID = *op.OutputValue
			}
		}
		md.cfg.Sync()

		if md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID == "" {
			md.lg.Warn("worker node security group ID not found")
			time.Sleep(5 * time.Second)
			continue
		}

		if md.cfg.EnableWorkerNodeSSH {
			md.lg.Info(
				"checking worker node group security group",
				zap.String("security-group-id", md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
			)
			var sout *ec2.DescribeSecurityGroupsOutput
			sout, err = md.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
				GroupIds: aws.StringSlice([]string{md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID}),
			})
			if err != nil {
				md.lg.Info("failed to describe worker node group security group",
					zap.String("stack-name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
					zap.String("stack-status", md.cfg.ClusterState.CFStackWorkerNodeGroupStatus),
					zap.String("security-group-id", md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
					zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
					zap.Error(err),
				)
				return err
			}
			if len(sout.SecurityGroups) < 1 {
				return fmt.Errorf(
					"expected at least 1 worker node group security group, got %d (%+v)",
					len(sout.SecurityGroups),
					sout.SecurityGroups,
				)
			}

			foundSSHAccess := false
		_foundSSHAccess:
			for _, sg := range sout.SecurityGroups {
				for _, perm := range sg.IpPermissions {
					if perm.FromPort == nil || perm.ToPort == nil {
						md.lg.Info(
							"found security IP permission",
							zap.String("security-group-id", md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
							zap.String("permission", fmt.Sprintf("%+v", perm)),
						)
						continue
					}
					fromPort, toPort := *perm.FromPort, *perm.ToPort
					rg := ""
					if len(perm.IpRanges) == 1 {
						rg = *perm.IpRanges[0].CidrIp
					}
					md.lg.Info(
						"found security IP permission",
						zap.String("security-group-id", md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
						zap.Int64("from-port", fromPort),
						zap.Int64("to-port", toPort),
						zap.String("cidr-ip", rg),
					)
					if fromPort == 22 && toPort == 22 && rg == "0.0.0.0/0" {
						foundSSHAccess = true
						break _foundSSHAccess
					}
				}
			}
			if !foundSSHAccess {
				md.lg.Warn("authorizing SSH access", zap.Int64("port", 22))
				_, aerr := md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:    aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
					IpProtocol: aws.String("tcp"),
					CidrIp:     aws.String("0.0.0.0/0"),
					FromPort:   aws.Int64(22),
					ToPort:     aws.Int64(22),
				})
				if aerr != nil {
					return aerr
				}
				md.lg.Info("authorized SSH access ingress", zap.Int64("port", 22))
			}
		}

		if md.cfg.EnableWorkerNodePrivilegedPortAccess {
			md.lg.Warn("authorizing worker node privileged port access for control plane", zap.String("port-range", "1-1024"))
			_, err = md.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
				GroupId:    aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupSecurityGroupID),
				IpProtocol: aws.String("tcp"),
				CidrIp:     aws.String("0.0.0.0/0"),
				FromPort:   aws.Int64(1),
				ToPort:     aws.Int64(1024),
			})
			if err != nil {
				return err
			}
			_, err = md.ec2.AuthorizeSecurityGroupEgress(&ec2.AuthorizeSecurityGroupEgressInput{
				GroupId: aws.String(md.cfg.SecurityGroupID),
				IpPermissions: []*ec2.IpPermission{
					{
						IpProtocol: aws.String("tcp"),
						FromPort:   aws.Int64(1),
						ToPort:     aws.Int64(1024),
						IpRanges: []*ec2.IpRange{
							{CidrIp: aws.String("0.0.0.0/0")},
						},
					},
				},
			})
			if err != nil {
				return err
			}
			md.lg.Warn("authorizing worker node privileged port access for control plane", zap.String("port-range", "1-1024"))
		}

		md.lg.Info(
			"worker node creation in progress",
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)

		if md.cfg.ClusterState.CFStackWorkerNodeGroupStatus == "CREATE_COMPLETE" {
			if err = md.checkASG(); err != nil {
				md.lg.Warn("failed to check ASG", zap.Error(err))
				continue
			}
			break
		}
		time.Sleep(15 * time.Second)
	}

	if err != nil {
		md.lg.Info("failed to create worker node",
			zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
			zap.String("stack-status", md.cfg.ClusterState.CFStackWorkerNodeGroupStatus),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	if md.cfg.ClusterState.CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN == "" {
		return errors.New("cannot find node group instance role ARN")
	}

	md.lg.Info("created worker node",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.String("stack-status", md.cfg.ClusterState.CFStackWorkerNodeGroupStatus),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	// write config map file
	var cmPath string
	cmPath, err = writeConfigMapNodeAuth(md.cfg.ClusterState.CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN)
	if err != nil {
		return err
	}
	defer os.RemoveAll(cmPath)

	applied := false
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		// TODO: use "k8s.io/client-go"
		if !applied {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var kexo []byte
			kexo, err = exec.New().CommandContext(ctx,
				md.cfg.KubectlPath,
				"--kubeconfig="+md.cfg.KubeConfigPath,
				"apply", "--filename="+cmPath,
			).CombinedOutput()
			cancel()
			if err != nil {
				if strings.Contains(err.Error(), "unknown flag:") {
					return fmt.Errorf("unknown flag %s", string(kexo))
				}
				md.lg.Warn("failed to apply config map",
					zap.String("stack-name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
					zap.String("output", string(kexo)),
					zap.Error(err),
				)
				md.cfg.ClusterState.WorkerNodeGroupStatus = err.Error()
				md.cfg.Sync()
				time.Sleep(5 * time.Second)
				continue
			}
			applied = true
			md.lg.Info("kubectl apply completed", zap.String("output", string(kexo)))
		}

		// TODO: use "k8s.io/client-go"
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var kexo []byte
		kexo, err = exec.New().CommandContext(ctx,
			md.cfg.KubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "nodes", "-ojson",
		).CombinedOutput()
		cancel()
		if err != nil {
			if strings.Contains(err.Error(), "unknown flag:") {
				return fmt.Errorf("unknown flag %s", string(kexo))
			}
			md.lg.Warn("failed to get nodes", zap.String("output", string(kexo)), zap.Error(err))
			md.cfg.ClusterState.WorkerNodeGroupStatus = err.Error()
			md.cfg.Sync()
			time.Sleep(5 * time.Second)
			continue
		}

		var ns *nodeList
		ns, err = kubectlGetNodes(kexo)
		if err != nil {
			md.lg.Warn("failed to parse get nodes output", zap.Error(err))
			md.cfg.ClusterState.WorkerNodeGroupStatus = err.Error()
			md.cfg.Sync()
			time.Sleep(10 * time.Second)
			continue
		}
		nodesN := len(ns.Items)
		readyN := countReadyNodes(ns)
		md.lg.Info(
			"created worker nodes",
			zap.Int("created-nodes", nodesN),
			zap.Int("ready-nodes", readyN),
			zap.Int("worker-node-asg-min", md.cfg.WorkerNodeASGMin),
			zap.Int("worker-node-asg-max", md.cfg.WorkerNodeASGMax),
		)
		if readyN == md.cfg.WorkerNodeASGMax {
			md.cfg.ClusterState.WorkerNodeGroupStatus = "READY"
			md.cfg.Sync()
			break
		}

		md.cfg.ClusterState.WorkerNodeGroupStatus = fmt.Sprintf("%d AVAILABLE", nodesN)
		md.cfg.Sync()

		time.Sleep(15 * time.Second)
	}

	if md.cfg.ClusterState.WorkerNodeGroupStatus != "READY" {
		return fmt.Errorf(
			"worker nodes are not ready (status %q, ASG max %d)",
			md.cfg.ClusterState.WorkerNodeGroupStatus,
			md.cfg.WorkerNodeASGMax,
		)
	}

	md.lg.Info(
		"enabled node group to join cluster",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteWorkerNode() error {
	if !md.cfg.ClusterState.StatusWorkerNodeCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusWorkerNodeCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterState.CFStackWorkerNodeGroupName == "" {
		return errors.New("cannot delete empty worker node")
	}

	_, err := md.cfn.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
	})
	if err != nil {
		md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = err.Error()
		md.cfg.ClusterState.WorkerNodeGroupStatus = err.Error()
		return err
	}

	md.cfg.Sync()

	md.lg.Info("waiting for 1-minute")
	time.Sleep(time.Minute)

	waitTime := 5*time.Minute + 2*time.Duration(md.cfg.WorkerNodeASGMax)*time.Minute
	md.lg.Info(
		"periodically fetching node stack status",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.String("stack-name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.Duration("duration", waitTime),
	)

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		var do *cloudformation.DescribeStacksOutput
		do, err = md.cfn.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		})
		if err == nil {
			md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = *do.Stacks[0].StackStatus
			md.cfg.ClusterState.WorkerNodeGroupStatus = *do.Stacks[0].StackStatus
			md.lg.Info(
				"deleting worker node stack",
				zap.String("stack-status", md.cfg.ClusterState.WorkerNodeGroupStatus),
				zap.String("request-started", humanize.RelTime(retryStart, time.Now().UTC(), "ago", "from now")),
			)
			time.Sleep(5 * time.Second)
			continue
		}

		if isCFDeletedGoClient(md.cfg.ClusterState.CFStackWorkerNodeGroupName, err) {
			err = nil
			md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = "DELETE_COMPLETE"
			md.cfg.ClusterState.WorkerNodeGroupStatus = "DELETE_COMPLETE"
			break
		}

		md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = err.Error()
		md.cfg.ClusterState.WorkerNodeGroupStatus = err.Error()

		md.lg.Warn("failed to describe worker node", zap.Error(err))
		md.cfg.Sync()
		time.Sleep(10 * time.Second)
	}

	if err != nil {
		md.lg.Info("failed to delete worker node", zap.String("request-started", humanize.RelTime(retryStart, time.Now().UTC(), "ago", "from now")), zap.Error(err))
		return err
	}

	md.lg.Info(
		"deleted worker node",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.String("request-started", humanize.RelTime(retryStart, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

// aws cloudformation describe-stack-resources --stack-name a8-eks-190226-mqitl-NODE-GROUP-STACK
// aws autoscaling describe-auto-scaling-groups --auto-scaling-group-names a8-eks-190226-mqitl-NODE-GROUP-STACK-NodeGroup-S70MNA2WIO4E
func (md *embedded) checkASG() (err error) {
	md.lg.Info("checking auto scaling groups for worker node instance information")

	var rout *cloudformation.DescribeStackResourcesOutput
	rout, err = md.cfn.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
	})
	if err != nil {
		return err
	}
	if len(rout.StackResources) == 0 {
		return fmt.Errorf("stack resources not found for %q", md.cfg.ClusterState.CFStackWorkerNodeGroupName)
	}
	for _, ro := range rout.StackResources {
		if *ro.ResourceType == "AWS::AutoScaling::AutoScalingGroup" {
			md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName = *ro.PhysicalResourceId
			md.lg.Info(
				"found worker node ASG name",
				zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName),
			)
			break
		}
	}
	if md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName == "" {
		return errors.New("can't find physical resource ID for ASG")
	}

	time.Sleep(5 * time.Second)

	var aout *autoscaling.DescribeAutoScalingGroupsOutput
	aout, err = md.asg.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: aws.StringSlice([]string{md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName}),
	})
	if err != nil {
		return fmt.Errorf("ASG not found for %q (%v)", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName, err)
	}
	if len(aout.AutoScalingGroups) != 1 {
		return fmt.Errorf("expected only 1 ASG, got %+v", aout.AutoScalingGroups)
	}
	asg := aout.AutoScalingGroups[0]

	if *asg.MinSize != int64(md.cfg.WorkerNodeASGMin) {
		return fmt.Errorf("ASG min size expected %d, got %d", md.cfg.WorkerNodeASGMin, *asg.MinSize)
	}
	if *asg.MaxSize != int64(md.cfg.WorkerNodeASGMax) {
		return fmt.Errorf("ASG max size expected %d, got %d", md.cfg.WorkerNodeASGMax, *asg.MaxSize)
	}
	if len(asg.Instances) != md.cfg.WorkerNodeASGMax {
		return fmt.Errorf("instances expected %d, got %d", md.cfg.WorkerNodeASGMax, len(asg.Instances))
	}
	healthCnt := 0
	for _, iv := range asg.Instances {
		if *iv.HealthStatus == "Healthy" {
			healthCnt++
		}
	}
	if healthCnt != len(asg.Instances) {
		return fmt.Errorf("instances health count expected %d, got %d", len(asg.Instances), healthCnt)
	}
	ids := make([]string, 0, len(asg.Instances))
	for _, iv := range asg.Instances {
		ids = append(ids, *iv.InstanceId)
	}

	time.Sleep(3 * time.Second)

	ec2Instances := make([]*ec2.Instance, 0, len(ids))
	// batch by 10
	for len(ids) > 0 {
		iss := ids
		if len(ids) > 10 {
			iss = ids[:10]
		}
		var dout *ec2.DescribeInstancesOutput
		dout, err = md.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(iss),
		})
		if err != nil {
			return fmt.Errorf("failed to describe instances %v", err)
		}

		runningCnt := 0
		for _, rsrv := range dout.Reservations {
			ec2Instances = append(ec2Instances, rsrv.Instances...)
			for _, iv := range rsrv.Instances {
				if *iv.State.Name == "running" {
					runningCnt++
				}
			}
		}
		if runningCnt != len(iss) {
			return fmt.Errorf("running instances expected %d, got %d", len(iss), runningCnt)
		}
		md.lg.Info("EC2 instances are running",
			zap.Int("reservations", len(dout.Reservations)),
			zap.Int("instances-so-far", len(ec2Instances)),
		)

		if len(ids) <= 10 {
			break
		}
		ids = ids[:10]
		time.Sleep(5 * time.Second)
	}

	md.cfg.ClusterState.WorkerNodes = make(map[string]ec2config.Instance)
	for _, v := range ec2Instances {
		md.cfg.ClusterState.WorkerNodes[*v.InstanceId] = internalec2.ConvertEC2Instance(v)
	}
	fmt.Println(md.cfg.SSHCommands())

	md.lg.Info(
		"checking auto scaling groups for worker node instance information",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName),
	)
	return nil
}

// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
const configMapNodeAuthTempl = `---
apiVersion: v1
kind: ConfigMap

metadata:
  name: aws-auth
  namespace: kube-system

data:
  mapRoles: |
    - rolearn: {{.WorkerNodeInstanceRoleARN}}
      %s
      groups:
      - system:bootstrappers
      - system:nodes

`

type configMapNodeAuth struct {
	WorkerNodeInstanceRoleARN string
}

func writeConfigMapNodeAuth(arn string) (p string, err error) {
	kc := configMapNodeAuth{WorkerNodeInstanceRoleARN: arn}
	tpl := template.Must(template.New("configMapNodeAuthTempl").Parse(configMapNodeAuthTempl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, kc); err != nil {
		return "", err
	}
	// avoid '{{' conflicts with Go
	txt := fmt.Sprintf(buf.String(), `username: system:node:{{EC2PrivateDNSName}}`)
	return fileutil.WriteTempFile([]byte(txt))
}

// TODO: use k8s.io/client-go to list nodes

// reference: https://github.com/kubernetes/test-infra/blob/master/kubetest/kubernetes.go

// kubectlGetNodes lists nodes by executing kubectl get nodes, parsing the output into a nodeList object
func kubectlGetNodes(out []byte) (*nodeList, error) {
	nodes := &nodeList{}
	if err := json.Unmarshal(out, nodes); err != nil {
		return nil, fmt.Errorf("error parsing kubectl get nodes output: %v", err)
	}
	return nodes, nil
}

// isReady checks if the node has a Ready Condition that is True
func isReady(node *node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// countReadyNodes returns the number of nodes that have isReady == true
func countReadyNodes(nodes *nodeList) int {
	var ns []*node
	for i := range nodes.Items {
		nd := &nodes.Items[i]
		if isReady(nd) {
			ns = append(ns, nd)
		}
	}
	return len(ns)
}

// nodeList is a simplified version of the v1.NodeList API type
type nodeList struct {
	Items []node `json:"items"`
}

// node is a simplified version of the v1.Node API type
type node struct {
	Metadata metadata   `json:"metadata"`
	Status   nodeStatus `json:"status"`
}

// nodeStatus is a simplified version of the v1.NodeStatus API type
type nodeStatus struct {
	Addresses  []nodeAddress   `json:"addresses"`
	Conditions []nodeCondition `json:"conditions"`
}

// nodeAddress is a simplified version of the v1.NodeAddress API type
type nodeAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// nodeCondition is a simplified version of the v1.NodeCondition API type
type nodeCondition struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Type    string `json:"type"`
}

// metadata is a simplified version of the kubernetes metadata types
type metadata struct {
	Name string `json:"name"`
}
