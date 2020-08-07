// Package ec2 implements various AWS EC2 operations.
package ec2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/ctxutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
)

// WaitUntilRunning describes all EC2 instances for the specified ASG.
// It waits until all instances are 'running'.
// TODO: make method (e.g. asgClient."WaitUntilRunning")
func WaitUntilRunning(
	ctx context.Context,
	stopc chan struct{},
	asgAPI autoscalingiface.AutoScalingAPI,
	ec2API ec2iface.EC2API,
	asgName string) (ec2Instances map[string]*ec2.Instance, err error) {
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
		var aout *autoscaling.DescribeAutoScalingGroupsOutput
		aout, err = asgAPI.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: aws.StringSlice([]string{asgName}),
		})
		if err != nil {
			return nil, fmt.Errorf("ASG[%q] not found (%v)", asgName, err)
		}
		if len(aout.AutoScalingGroups) != 1 {
			return nil, fmt.Errorf("expected 1 ASG[%q], got %+v", asgName, aout.AutoScalingGroups)
		}
		av := aout.AutoScalingGroups[0]
		instanceIDs := make([]string, 0, len(av.Instances))
		for _, iv := range av.Instances {
			lv := aws.StringValue(iv.LifecycleState)
			switch lv {
			case autoscaling.LifecycleStatePending,
				autoscaling.LifecycleStatePendingWait,
				autoscaling.LifecycleStatePendingProceed,
				autoscaling.LifecycleStateInService:
				instanceIDs = append(instanceIDs, aws.StringValue(iv.InstanceId))
			default:
				zap.L().Warn("skipping instance due to lifecycle state",
					zap.String("instance-id", aws.StringValue(iv.InstanceId)),
					zap.String("lifecycle-state", lv),
				)
			}
		}

		// 25-minute for 500 nodes
		waitDur := 3 * time.Second * time.Duration(len(instanceIDs))
		ctx2, cancel := context.WithTimeout(ctx, waitDur)
		ec2Instances, err = pollUntilRunning(ctx2, stopc, ec2API, instanceIDs...)
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
	ec2API ec2iface.EC2API,
	instanceIDs ...string) (ec2Instances map[string]*ec2.Instance, err error) {
	targetN := len(instanceIDs)
	ec2Instances = make(map[string]*ec2.Instance, targetN)

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
		var dout *ec2.DescribeInstancesOutput
		dout, err = ec2API.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(batch),
		})
		if err != nil {
			zap.L().Warn("failed to describe instances", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rsrv := range dout.Reservations {
			for _, iv := range rsrv.Instances {
				state := aws.StringValue(iv.State.Name)
				if state != ec2.InstanceStateNameRunning {
					continue
				}
				instanceID := aws.StringValue(iv.InstanceId)
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
