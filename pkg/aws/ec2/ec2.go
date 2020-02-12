// Package ec2 implements various AWS EC2 operations.
package ec2

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
)

// PollUntilRunning describes EC2 instances by batch,
// and waits until all instances are 'running'.
func PollUntilRunning(
	timeout time.Duration,
	lg *zap.Logger,
	ec2API ec2iface.EC2API,
	instanceIDs ...string) (ec2Instances map[string]*ec2.Instance, err error) {
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < timeout {
		ec2Instances, err = describeByBatch(lg, ec2API, instanceIDs...)
		if err == nil {
			return ec2Instances, nil
		}
		lg.Warn("failed to describe instances", zap.Error(err))
		time.Sleep(10 * time.Second)
	}
	return ec2Instances, err
}

func describeByBatch(
	lg *zap.Logger,
	ec2API ec2iface.EC2API,
	instanceIDs ...string) (ec2Instances map[string]*ec2.Instance, err error) {
	ec2Instances = make(map[string]*ec2.Instance, len(instanceIDs))

	last := make([]string, len(instanceIDs))
	copy(last, instanceIDs)

	// batch by 10
	for len(last) > 0 {
		batch := last
		if len(last) > 10 {
			batch = last[:10]
		}
		var dout *ec2.DescribeInstancesOutput
		dout, err = ec2API.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(batch),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances %v", err)
		}

		runningCnt := 0
		for _, rsrv := range dout.Reservations {
			for _, iv := range rsrv.Instances {
				if aws.StringValue(iv.State.Name) == ec2.InstanceStateNameRunning {
					runningCnt++
				}
				ec2Instances[aws.StringValue(iv.InstanceId)] = iv
			}
		}
		if runningCnt != len(batch) {
			return nil, fmt.Errorf("running instances expected %d, got %d", len(batch), runningCnt)
		}
		lg.Info("EC2 instances are running",
			zap.Int("reservations", len(dout.Reservations)),
			zap.Int("instances-so-far", len(ec2Instances)),
		)

		if len(last) <= 10 {
			break
		}
		last = last[10:]
		time.Sleep(5 * time.Second)
	}

	return ec2Instances, nil
}
