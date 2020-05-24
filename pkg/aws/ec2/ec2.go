// Package ec2 implements various AWS EC2 operations.
package ec2

import (
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
	targetN := len(instanceIDs)
	ec2Instances = make(map[string]*ec2.Instance, targetN)

	left := make(map[string]struct{}, targetN)
	for _, id := range instanceIDs {
		left[id] = struct{}{}
	}

	lg.Info("polling instance status", zap.Int("target-total", targetN))
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < timeout {
		// batch by 30
		batch := make([]string, 0, 30)
		for id := range left {
			batch = append(batch, id)
			if len(batch) >= 30 {
				break
			}
		}

		lg.Info("describing batch", zap.Int("batch-total", len(batch)))
		var dout *ec2.DescribeInstancesOutput
		dout, err = ec2API.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: aws.StringSlice(batch),
		})
		if err != nil {
			lg.Warn("failed to describe instances", zap.Error(err))
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
		lg.Info("checking ec2 instances",
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
		time.Sleep(10 * time.Second)
	}

	return ec2Instances, err
}
