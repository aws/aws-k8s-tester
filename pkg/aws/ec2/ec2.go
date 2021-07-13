// Package ec2 implements various AWS EC2 operations.
package ec2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/ctxutil"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_asg_v2 "github.com/aws/aws-sdk-go-v2/service/autoscaling"
	aws_asg_v2_types "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

// WaitUntilRunning describes all EC2 instances for the specified ASG.
// It waits until all instances are 'running'.
// TODO: make method (e.g. asgClient."WaitUntilRunning")
func WaitUntilRunning(
	ctx context.Context,
	stopc chan struct{},
	ec2APIV2 *aws_ec2_v2.Client,
	asgAPIV2 *aws_asg_v2.Client,
	asgName string) (ec2Instances map[string]aws_ec2_v2_types.Instance, err error) {
	zap.L().Info("polling ASG until all EC2 instances are running",
		zap.String("asg-name", asgName),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
	)

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-stopc:
			return nil, errors.New("poll aborted")
		case <-time.After(10 * time.Second):
		}

		// When ASG has >500 nodes, some instances may shut down at any moments,
		// making previous instance ID list stale
		// thus, fetch latest instance IDs for every iteration
		aout, err := asgAPIV2.DescribeAutoScalingGroups(
			context.Background(),
			&aws_asg_v2.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: []string{asgName},
			},
		)
		if err != nil {
			return nil, fmt.Errorf("ASG[%q] not found (%v)", asgName, err)
		}
		if len(aout.AutoScalingGroups) != 1 {
			return nil, fmt.Errorf("expected 1 ASG[%q], got %+v", asgName, aout.AutoScalingGroups)
		}
		av := aout.AutoScalingGroups[0]
		instanceIDs := make([]string, 0, len(av.Instances))
		for _, iv := range av.Instances {
			lv := iv.LifecycleState
			switch lv {
			case aws_asg_v2_types.LifecycleStatePending,
				aws_asg_v2_types.LifecycleStatePendingWait,
				aws_asg_v2_types.LifecycleStatePendingProceed,
				aws_asg_v2_types.LifecycleStateInService:
				instanceIDs = append(instanceIDs, aws_v2.ToString(iv.InstanceId))
			default:
				zap.L().Warn("skipping instance due to lifecycle state",
					zap.String("instance-id", aws_v2.ToString(iv.InstanceId)),
					zap.String("lifecycle-state", fmt.Sprint(lv)),
				)
			}
		}

		// minimum waits for small number of node + extra waits for large number of nodes
		// e.g. 5-minute + 25-minute for 500 nodes
		waitDur := 5*time.Minute + 3*time.Second*time.Duration(len(instanceIDs))
		ctx2, cancel := context.WithTimeout(ctx, waitDur)
		ec2Instances, err = pollUntilRunning(ctx2, stopc, ec2APIV2, instanceIDs...)
		cancel()
		if err == nil {
			break
		}
		zap.L().Warn("failed to poll instance status; retrying", zap.Error(err))
	}
	return ec2Instances, err
}

// pollUntilRunning describes EC2 instances by batch,
// and waits until all instances are 'running'.
func pollUntilRunning(
	ctx context.Context,
	stopc chan struct{},
	ec2APIV2 *aws_ec2_v2.Client,
	instanceIDs ...string) (ec2Instances map[string]aws_ec2_v2_types.Instance, err error) {
	targetN := len(instanceIDs)
	ec2Instances = make(map[string]aws_ec2_v2_types.Instance, targetN)

	left := make(map[string]struct{}, targetN)
	for _, id := range instanceIDs {
		left[id] = struct{}{}
	}

	zap.L().Info("polling instance status",
		zap.Int("target-total", targetN),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
	)

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-stopc:
			return nil, errors.New("poll aborted")
		case <-time.After(10 * time.Second):
		}

		// batch by 30
		batch := make([]string, 0, 30)
		for id := range left {
			batch = append(batch, id)
			if len(batch) >= 30 {
				break
			}
		}

		zap.L().Info("describing batch", zap.Int("batch-total", len(batch)))
		dout, err := ec2APIV2.DescribeInstances(
			context.Background(),
			&aws_ec2_v2.DescribeInstancesInput{
				InstanceIds: batch,
			})
		if err != nil {
			zap.L().Warn("failed to describe instances", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rsrv := range dout.Reservations {
			for _, iv := range rsrv.Instances {
				state := iv.State.Name
				if state != aws_ec2_v2_types.InstanceStateNameRunning {
					continue
				}
				instanceID := aws_v2.ToString(iv.InstanceId)
				ec2Instances[instanceID] = iv
			}
		}
		zap.L().Info("checking ec2 instances",
			zap.Int("reservations", len(dout.Reservations)),
			zap.Int("running-instances-so-far", len(ec2Instances)),
			zap.Int("target-total", targetN),
		)

		// remove completed instances from next batch
		for id := range ec2Instances {
			delete(left, id)
		}
		if len(left) == 0 {
			break
		}
	}

	return ec2Instances, err
}
