package cloud

import (
	"context"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	log "github.com/cihub/seelog"
	"k8s.io/apimachinery/pkg/util/wait"
)

// AutoScaling is an wrapper around the original AutoscalingAPI with additional convenient APIs.
type AutoScaling interface {
	autoscalingiface.AutoScalingAPI
	DescribeAutoScalingGroupsAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Group, error)
	DescribeAutoScalingGroupInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Instance, error)
	DescribeInServiceAutoScalingGroupInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Instance, error)
	WaitUntilAutoScalingGroupInService(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) error
	DescribeAutoScalingInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingInstancesInput) ([]*autoscaling.InstanceDetails, error)
}

// NewAutoScaling creates a new autoscaling session
func NewAutoScaling(session *session.Session) AutoScaling {
	return &defaultAutoScaling{
		autoscaling.New(session),
	}
}

var _ AutoScaling = (*defaultAutoScaling)(nil)

type defaultAutoScaling struct {
	autoscalingiface.AutoScalingAPI
}

func (c *defaultAutoScaling) DescribeAutoScalingGroupsAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Group, error) {
	var result []*autoscaling.Group
	if err := c.DescribeAutoScalingGroupsPagesWithContext(ctx, input, func(output *autoscaling.DescribeAutoScalingGroupsOutput, _ bool) bool {
		result = append(result, output.AutoScalingGroups...)
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *defaultAutoScaling) DescribeAutoScalingGroupInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Instance, error) {
	var result []*autoscaling.Instance
	if err := c.DescribeAutoScalingGroupsPagesWithContext(ctx, input, func(output *autoscaling.DescribeAutoScalingGroupsOutput, _ bool) bool {
		for _, item := range output.AutoScalingGroups {
			result = append(result, item.Instances...)
		}
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *defaultAutoScaling) DescribeInServiceAutoScalingGroupInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) ([]*autoscaling.Instance, error) {
	var instances []*autoscaling.Instance
	var result []*autoscaling.Instance
	if err := c.DescribeAutoScalingGroupsPagesWithContext(ctx, input, func(output *autoscaling.DescribeAutoScalingGroupsOutput, _ bool) bool {
		for _, item := range output.AutoScalingGroups {
			instances = append(instances, item.Instances...)
		}
		for _, instance := range instances {
			if *(instance.LifecycleState) == autoscaling.LifecycleStateInService {
				result = append(result, instance)
			}
		}
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// WaitUntilAutoScalingGroupInService waits until the ASG has the number of instances InService == DesiredCapacity
// TODO: probably make this only deal with one ASG at a time because it will loop through an ASG that is already all in service again
func (c *defaultAutoScaling) WaitUntilAutoScalingGroupInService(ctx context.Context, input *autoscaling.DescribeAutoScalingGroupsInput) error {
	return wait.PollImmediateUntil(utils.PollIntervalMedium, func() (bool, error) {
		asgOut, err := c.DescribeAutoScalingGroupsAsList(ctx, input)
		if err != nil {
			return false, err
		}
		for i, asg := range asgOut {
			var count int64
			for _, instance := range asg.Instances {
				log.Debugf("Instance (%s) state %s", *(instance.InstanceId), *(instance.LifecycleState))
				if *(instance.LifecycleState) == autoscaling.LifecycleStateInService {
					count++
				}
				if count >= *asg.DesiredCapacity {
					break
				}
			}
			if count < *asg.DesiredCapacity {
				log.Debugf("Number of instances InService (%d) is less than DesiredCapacity (%d)", count, *asg.DesiredCapacity)
				break
			}
			if len(asgOut)-1 == i {
				return true, nil
			}
		}
		return false, nil
	}, ctx.Done())
}

func (c *defaultAutoScaling) DescribeAutoScalingInstancesAsList(ctx context.Context, input *autoscaling.DescribeAutoScalingInstancesInput) ([]*autoscaling.InstanceDetails, error) {
	var result []*autoscaling.InstanceDetails
	if err := c.DescribeAutoScalingInstancesPagesWithContext(ctx, input, func(output *autoscaling.DescribeAutoScalingInstancesOutput, _ bool) bool {
		result = append(result, output.AutoScalingInstances...)
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}
