// Package elb implements ELB utilities.
package elb

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
)

// DeleteELBv2 deletes all resources associated
// with the load balancer.
// TODO: is there a better way to clean up resources?
// ref. https://github.com/aws/aws-k8s-tester/issues/70
func DeleteELBv2(lg *zap.Logger, elb2API elbv2iface.ELBV2API, arn string) error {
	if arn == "" {
		return errors.New("empty ELB ARN")
	}
	// deleteInOrder deletes listeners, target groups, and ELB in order.
	if err := deleteInOrder(lg, elb2API, arn); err == nil {
		lg.Info("successfully deleted ELB in order")
	}
	// deleteInReverseOrder deletes ELB and expects ENIs to be detached and deleted.
	if err := deleteInReverseOrder(lg, elb2API, arn); err == nil {
		lg.Info("successfully deleted ELB in reverse order")
	}
	return nil
}

// deleteInOrder deletes listeners, target groups, and ELB in order.
func deleteInOrder(lg *zap.Logger, elb2API elbv2iface.ELBV2API, arn string) error {
	lbDeleted := false
	// delete listener first
	// e.g. ResourceInUse: Target group is currently in use by a listener or a rule
	lg.Info("describing listeners", zap.String("arn", arn))
	ls, err := elb2API.DescribeListeners(&elbv2.DescribeListenersInput{
		LoadBalancerArn: aws.String(arn),
	})
	if err != nil {
		if !isDeleted(err) {
			return err
		}
		lbDeleted = true
		lg.Info("load balancer has already been deleted", zap.Error(err))
	}
	if !lbDeleted && ls != nil && len(ls.Listeners) > 0 {
		for _, lv := range ls.Listeners {
			arn := aws.StringValue(lv.ListenerArn)

			lg.Info("describing rules for listener", zap.String("listener-arn", arn))
			ro, err := elb2API.DescribeRules(&elbv2.DescribeRulesInput{
				ListenerArn: lv.ListenerArn,
			})
			if err != nil {
				lg.Warn("failed to describe rules for listener", zap.Error(err))
			} else {
				for _, rv := range ro.Rules {
					ruleArn := aws.StringValue(rv.RuleArn)
					lg.Info("deleting rule for listener", zap.String("rule-arn", ruleArn))
					_, err = elb2API.DeleteRule(&elbv2.DeleteRuleInput{
						RuleArn: rv.RuleArn,
					})
					if err != nil {
						lg.Info("failed to delete rule for listener", zap.String("rule-arn", ruleArn), zap.Error(err))
					} else {
						lg.Info("deleted rule for listener",
							zap.String("listener-arn", arn),
							zap.String("rule-arn", ruleArn),
						)
					}
				}
			}

			lg.Info("deleting listener", zap.String("listener-arn", arn))
			_, err = elb2API.DeleteListener(&elbv2.DeleteListenerInput{
				ListenerArn: lv.ListenerArn,
			})
			if err != nil {
				lg.Warn("failed to delete listener", zap.String("listener-arn", arn), zap.Error(err))
			} else {
				lg.Info("deleted listener", zap.String("listener-arn", arn))
			}
		}
	}

	lg.Info("describing target groups", zap.String("arn", arn))
	to, err := elb2API.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
		LoadBalancerArn: aws.String(arn),
	})
	if err != nil {
		if !isDeleted(err) {
			return err
		}
		lbDeleted = true
		lg.Info("load balancer has already been deleted", zap.Error(err))
	}
	if !lbDeleted && to != nil && len(to.TargetGroups) > 0 {
		for _, tv := range to.TargetGroups {
			arn := aws.StringValue(tv.TargetGroupArn)
			name := aws.StringValue(tv.TargetGroupName)
			tp := aws.StringValue(tv.TargetType)
			lg.Info("deleting target group",
				zap.String("arn", arn),
				zap.String("name", name),
				zap.String("type", tp),
			)
			_, err = elb2API.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{
				TargetGroupArn: tv.TargetGroupArn,
			})
			if err != nil {
				lg.Warn("failed to delete target group", zap.Error(err))
			} else {
				lg.Info("deleted target group")
			}
		}
	}

	err = nil
	if !lbDeleted {
		for i := 0; i < 5; i++ {
			time.Sleep(10 * time.Second)
			lg.Info("deleting ELB in order", zap.String("arn", arn))
			_, err = elb2API.DeleteLoadBalancer(&elbv2.DeleteLoadBalancerInput{
				LoadBalancerArn: aws.String(arn),
			})
			if err == nil {
				lg.Info("successfully deleted ELB in order")
				lbDeleted = true
				break
			}
			if isDeleted(err) {
				err, lbDeleted = nil, true
				lg.Info("ELB has already been deleted", zap.Error(err))
				break
			}
			lg.Warn("failing to delete ELB in order", zap.Error(err))
		}
	}
	if err == nil && lbDeleted {
		lg.Info("deleted ELB in order")
		return nil
	}
	if err != nil && lbDeleted {
		return nil
	}

	if err != nil && !lbDeleted {
		lg.Warn("failed to delete ELB in order", zap.Error(err))
	}
	return err
}

// deleteInReverseOrder deletes ELB and expects ENIs to be detached and deleted.
func deleteInReverseOrder(lg *zap.Logger, elb2API elbv2iface.ELBV2API, arn string) error {
	var err error
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Second)
		lg.Info("deleting ELB in reverse order", zap.String("arn", arn))
		_, err = elb2API.DeleteLoadBalancer(&elbv2.DeleteLoadBalancerInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err == nil {
			lg.Info("successfully deleted ELB in reverse order")
			lg.Info("waiting for ENI clean up after ELB deletion")
			time.Sleep(30 * time.Second)
			break
		}
		if isDeleted(err) {
			lg.Info("ELB has already been deleted in order", zap.Error(err))
			break
		}
		lg.Warn("failing to delete ELB in order", zap.Error(err))
	}
	return err
}

func isDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	return awsErr.Code() == "LoadBalancerNotFound"
}
