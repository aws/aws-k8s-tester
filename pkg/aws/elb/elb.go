// Package elb implements ELB utilities.
package elb

import (
	"context"
	"errors"
	"fmt"
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
func DeleteELBv2(lg *zap.Logger, elb2API elbv2iface.ELBV2API, arn string, vpcID string, tags map[string]string) (err error) {
	recurse := false
	if arn == "" {
		lg.Info("load balancer ARN not given, querying by VPC and cluster name",
			zap.String("vpc-id", vpcID),
			zap.String("tags", fmt.Sprintf("%+v", tags)),
		)
		elbARNs := make([]string, 0)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err = elb2API.DescribeLoadBalancersPagesWithContext(
			ctx,
			&elbv2.DescribeLoadBalancersInput{},
			func(output *elbv2.DescribeLoadBalancersOutput, _ bool) bool {
				for _, ev := range output.LoadBalancers {
					arn := aws.StringValue(ev.LoadBalancerArn)
					vpcID := aws.StringValue(ev.VpcId)
					if vpcID == vpcID {
						lg.Warn("found ELBv2 for this VPC",
							zap.String("vpc-id", vpcID),
							zap.String("elb-arn", arn),
						)
						elbARNs = append(elbARNs, arn)
					} else {
						lg.Info("found ELBv2 for other VPCs", zap.String("vpc-id", vpcID), zap.String("elb-arn", arn))
					}
				}
				return true
			})
		cancel()
		if err != nil {
			lg.Warn("failed to describe ELBv2", zap.Error(err))
			return errors.New("empty ELB ARN")
		}

		lg.Info("describing tags for elb", zap.Strings("elb-arns", elbARNs))
		tout, err := elb2API.DescribeTags(&elbv2.DescribeTagsInput{ResourceArns: aws.StringSlice(elbARNs)})
		if err != nil {
			lg.Warn("failed to describe tags", zap.Error(err))
		}
		matchingARNs := make([]string, 0)
		for _, desc := range tout.TagDescriptions {
			copied := make(map[string]string)
			for k, v := range tags {
				copied[k] = v
			}
			elbARN := aws.StringValue(desc.ResourceArn)
			for _, tv := range desc.Tags {
				k, v1 := aws.StringValue(tv.Key), aws.StringValue(tv.Value)
				lg.Info("found tag", zap.String("elb-arn", elbARN), zap.String("key", k), zap.String("value", v1))
				if v2, ok := copied[k]; ok && v2 == v1 {
					delete(copied, k)
					lg.Info("found matching tag", zap.String("key", k), zap.String("value", v1))
				}
			}
			if len(copied) == 0 {
				lg.Info("found elb with matching tags; deleting", zap.String("elb-arn", elbARN))
				matchingARNs = append(matchingARNs, elbARN)
			}
		}

		lg.Info("matching elb", zap.Strings("elb-arns", matchingARNs))
		switch {
		case len(matchingARNs) == 0:
			lg.Warn("could not found matching elb")
			return errors.New("empty ELB ARN")
		case len(matchingARNs) == 1:
			lg.Warn("found 1 matching elb")
			arn = matchingARNs[0]
		case len(matchingARNs) == 2:
			lg.Warn("found 2 matching elb")
			arn = matchingARNs[0]
			recurse = true
			lg.Info("need recursively delete elb", zap.String("elb-arn", matchingARNs[1]))
		}
	}

	// deleteInOrder deletes listeners, target groups, and ELB in order.
	if err = deleteInOrder(lg, elb2API, arn); err == nil {
		lg.Info("successfully deleted ELB in order")
	}
	// deleteInReverseOrder deletes ELB and expects ENIs to be detached and deleted.
	if err = deleteInReverseOrder(lg, elb2API, arn); err == nil {
		lg.Info("successfully deleted ELB in reverse order")
	}
	if recurse {
		// pass empty, so it can fetch recursively len(elbs) > 2
		return DeleteELBv2(lg, elb2API, "", vpcID, tags)
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
