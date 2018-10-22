package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/awstester/eksconfig"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) createWorkerNode() error {
	if md.cfg.ClusterState.CFStackWorkerNodeGroupKeyPairName == "" {
		return errors.New("cannot create worker node without key name")
	}
	if md.cfg.ClusterState.CFStackWorkerNodeGroupName == "" {
		return errors.New("cannot create empty worker node")
	}

	now := time.Now().UTC()
	h, _ := os.Hostname()

	s, err := createWorkerNodeTemplateFromURL(md.lg)
	if err != nil {
		return err
	}

	_, err = md.cf.CreateStack(&cloudformation.CreateStackInput{
		StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String(md.cfg.Tag),
				Value: aws.String(md.cfg.ClusterName),
			},
			{
				Key:   aws.String("HOSTNAME"),
				Value: aws.String(h),
			},
		},

		// TemplateURL: aws.String("https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-nodegroup.yaml"),
		TemplateBody: aws.String(s),

		Parameters: []*cloudformation.Parameter{
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
				ParameterValue: aws.String(md.cfg.WorkerNodeAMI),
			},
			{
				ParameterKey:   aws.String("NodeInstanceType"),
				ParameterValue: aws.String(md.cfg.WorkerNodeInstanceType),
			},
			{
				ParameterKey:   aws.String("NodeAutoScalingGroupMinSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkderNodeASGMin)),
			},
			{
				ParameterKey:   aws.String("NodeAutoScalingGroupMaxSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkderNodeASGMax)),
			},
			{
				ParameterKey:   aws.String("NodeVolumeSize"),
				ParameterValue: aws.String(fmt.Sprintf("%d", md.cfg.WorkerNodeVolumeSizeGB)),
			},
			{
				ParameterKey:   aws.String("VpcId"),
				ParameterValue: aws.String(md.cfg.ClusterState.CFStackVPCID),
			},
			{
				ParameterKey:   aws.String("Subnets"),
				ParameterValue: aws.String(strings.Join(md.cfg.ClusterState.CFStackVPCSubnetIDs, ",")),
			},
			{
				ParameterKey:   aws.String("ClusterControlPlaneSecurityGroup"),
				ParameterValue: aws.String(md.cfg.ClusterState.CFStackVPCSecurityGroupID),
			},
		},

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

	waitTime := 7*time.Minute + 2*time.Duration(md.cfg.WorkderNodeASGMax)*time.Minute
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		var do *cloudformation.DescribeStacksOutput
		do, err = md.cf.DescribeStacks(&cloudformation.DescribeStacksInput{
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

		if md.cfg.EnableNodeSSH {
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
						rg = perm.IpRanges[0].String()
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

		md.lg.Info(
			"worker node creation in progress",
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)
		if md.cfg.ClusterState.CFStackWorkerNodeGroupStatus == "CREATE_COMPLETE" {
			if err = md.updateASG(); err != nil {
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

	kcfgPath := md.cfg.KubeConfigPath
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
			cmd := md.kubectl.CommandContext(ctx,
				md.kubectlPath,
				"--kubeconfig="+kcfgPath,
				"apply", "--filename="+cmPath,
			)
			var kexo []byte
			kexo, err = cmd.CombinedOutput()
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
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "nodes", "-ojson",
		)
		var kexo []byte
		kexo, err = cmd.CombinedOutput()
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
			zap.Int("worker-node-asg-min", md.cfg.WorkderNodeASGMin),
			zap.Int("worker-node-asg-max", md.cfg.WorkderNodeASGMax),
		)
		if readyN == md.cfg.WorkderNodeASGMax {
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
			md.cfg.WorkderNodeASGMax,
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

	_, err := md.cf.DeleteStack(&cloudformation.DeleteStackInput{
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

	waitTime := 5*time.Minute + 2*time.Duration(md.cfg.WorkderNodeASGMax)*time.Minute
	md.lg.Info(
		"periodically fetching node stack status",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.String("stack-name", md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		zap.Duration("duration", waitTime),
	)

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		var do *cloudformation.DescribeStacksOutput
		do, err = md.cf.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(md.cfg.ClusterState.CFStackWorkerNodeGroupName),
		})
		if err == nil {
			md.cfg.ClusterState.CFStackWorkerNodeGroupStatus = *do.Stacks[0].StackStatus
			md.cfg.ClusterState.WorkerNodeGroupStatus = *do.Stacks[0].StackStatus
			md.lg.Info("deleting worker node stack", zap.String("request-started", humanize.RelTime(retryStart, time.Now().UTC(), "ago", "from now")))
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

func (md *embedded) updateASG() (err error) {
	md.lg.Info("checking ASG")

	var rout *cloudformation.DescribeStackResourcesOutput
	rout, err = md.cf.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
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

	if *asg.MinSize != int64(md.cfg.WorkderNodeASGMin) {
		return fmt.Errorf("ASG min size expected %d, got %d", md.cfg.WorkderNodeASGMin, *asg.MinSize)
	}
	if *asg.MaxSize != int64(md.cfg.WorkderNodeASGMax) {
		return fmt.Errorf("ASG max size expected %d, got %d", md.cfg.WorkderNodeASGMax, *asg.MaxSize)
	}
	if len(asg.Instances) != md.cfg.WorkderNodeASGMax {
		return fmt.Errorf("instances expected %d, got %d", md.cfg.WorkderNodeASGMax, len(asg.Instances))
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
		if len(dout.Reservations) != 1 {
			return fmt.Errorf("ec2 DescribeInstances returned len(Reservations) %d", len(dout.Reservations))
		}
		ec2Instances = append(ec2Instances, dout.Reservations[0].Instances...)

		runningCnt := 0
		for _, iv := range dout.Reservations[0].Instances {
			if *iv.State.Name == "running" {
				runningCnt++
			}
		}
		if runningCnt != len(dout.Reservations[0].Instances) {
			return fmt.Errorf("running instances expected %d, got %d", len(dout.Reservations[0].Instances), runningCnt)
		}
		md.lg.Info("EC2 instances are running", zap.Int("instances-so-far", len(ec2Instances)))

		if len(ids) <= 10 {
			break
		}
		ids = ids[:10]
		time.Sleep(5 * time.Second)
	}

	md.ec2InstancesMu.Lock()
	md.ec2Instances = ec2Instances
	md.cfg.ClusterState.WorkerNodes = ConvertEC2Instances(ec2Instances)
	md.ec2InstancesMu.Unlock()

	md.lg.Info(
		"checked ASG",
		zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName),
	)
	return nil
}

// ConvertEC2Instances converts "aws ec2 describe-instances" to "eksconfig.Instance".
func ConvertEC2Instances(iss []*ec2.Instance) (instances []eksconfig.Instance) {
	instances = make([]eksconfig.Instance, len(iss))
	for i, v := range iss {
		instances[i] = eksconfig.Instance{
			ImageID:      *v.ImageId,
			InstanceID:   *v.InstanceId,
			InstanceType: *v.InstanceType,
			KeyName:      *v.KeyName,
			Placement: eksconfig.EC2Placement{
				AvailabilityZone: *v.Placement.AvailabilityZone,
				Tenancy:          *v.Placement.Tenancy,
			},
			PrivateDNSName: *v.PrivateDnsName,
			PrivateIP:      *v.PrivateIpAddress,
			PublicDNSName:  *v.PublicDnsName,
			PublicIP:       *v.PublicIpAddress,
			EC2State: eksconfig.EC2State{
				Code: *v.State.Code,
				Name: *v.State.Name,
			},
			SubnetID:               *v.SubnetId,
			VPCID:                  *v.VpcId,
			EC2BlockDeviceMappings: make([]eksconfig.EC2BlockDeviceMapping, len(v.BlockDeviceMappings)),
			EBSOptimized:           *v.EbsOptimized,
			RootDeviceName:         *v.RootDeviceName,
			RootDeviceType:         *v.RootDeviceType,
			SecurityGroups:         make([]eksconfig.EC2SecurityGroup, len(v.SecurityGroups)),
		}
		for j := range v.BlockDeviceMappings {
			instances[i].EC2BlockDeviceMappings[j] = eksconfig.EC2BlockDeviceMapping{
				DeviceName: *v.BlockDeviceMappings[j].DeviceName,
				EBS: eksconfig.EBS{
					DeleteOnTermination: *v.BlockDeviceMappings[j].Ebs.DeleteOnTermination,
					Status:              *v.BlockDeviceMappings[j].Ebs.Status,
					VolumeID:            *v.BlockDeviceMappings[j].Ebs.VolumeId,
				},
			}
		}
		for j := range v.SecurityGroups {
			instances[i].SecurityGroups[j] = eksconfig.EC2SecurityGroup{
				GroupName: *v.SecurityGroups[j].GroupName,
				GroupID:   *v.SecurityGroups[j].GroupId,
			}
		}
	}
	return instances
}
