package alb

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"go.uber.org/zap"
)

func (md *embedded) TestAWSResources() error {
	md.lg.Info(
		"testing ALB resources",
		zap.Int("elbv2-number", len(md.cfg.ALBIngressController.ELBv2NameToARN)),
	)
	for name, arn := range md.cfg.ALBIngressController.ELBv2NameToARN {
		desc, err := md.elbv2.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			md.lg.Debug("failed to describe target group", zap.String("elbv2-name", name), zap.Error(err))
		} else {
			n := len(desc.TargetGroups)
			md.lg.Info("test found target groups", zap.String("elbv2-name", name), zap.Int("groups", n))
			if n == 0 {
				// TODO: return error
				md.lg.Warn("failed to describe target groups")
			}
		}
	}
	md.lg.Info(
		"tested ALB resources",
		zap.Int("elbv2-number", len(md.cfg.ALBIngressController.ELBv2NameToARN)),
	)
	return nil
}
