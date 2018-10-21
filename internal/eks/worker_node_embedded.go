package eks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/autoscaling"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
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
	v := workerNodeStack{
		Description:   md.cfg.ClusterName + "-worker-node-stack",
		TagKey:        md.cfg.Tag,
		TagValue:      md.cfg.ClusterName,
		Hostname:      h,
		EnableNodeSSH: md.cfg.EnableNodeSSH,
	}
	s, err := createWorkerNodeTemplate(v)
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

	waitTime := 5*time.Minute + 2*time.Duration(md.cfg.WorkderNodeASGMax)*time.Minute
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
			time.Sleep(5 * time.Second)
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

		for _, op := range do.Stacks[0].Outputs {
			if *op.OutputKey == "NodeInstanceRole" {
				md.lg.Info("found NodeInstanceRole", zap.String("output", *op.OutputValue))
				md.cfg.ClusterState.CFStackWorkerNodeGroupWorkerNodeInstanceRoleARN = *op.OutputValue
			}
		}

		md.cfg.Sync()
		md.lg.Info("creating worker node", zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")))

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
			md.lg.Info("deleting worker node", zap.String("request-started", humanize.RelTime(retryStart, time.Now().UTC(), "ago", "from now")))
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

	md.lg.Info("e2e testing ASG", zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName))

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
	// md.instances = asg.Instances

	md.lg.Info("e2e tested ASG", zap.String("name", md.cfg.ClusterState.CFStackWorkerNodeGroupAutoScalingGroupName))
	return nil
}
