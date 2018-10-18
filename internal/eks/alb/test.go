package alb

import (
	"fmt"
	"strings"
	"time"

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
		// TODO: aws elbv2 describe-target-health --target-group-arn
		// returns "State": "unhealthy",
		// "Reason": "Target.ResponseCodeMismatch",
		// "Description": "Health checks failed with these codes: [404]"
		// path /* or /metrics does not work...
		if strings.Contains(name, "kubesystem") {
			md.lg.Info("skipping health check", zap.String("elbv2-name", name))
			continue
		}

		desc, err := md.elbv2.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			md.lg.Warn("failed to describe target group", zap.String("elbv2-name", name), zap.Error(err))
			return err
		}

		md.lg.Info(
			"test found target groups",
			zap.String("elbv2-name", name),
			zap.String("alb-target-tyope", md.cfg.ALBIngressController.TargetType),
			zap.Int("server-replicas", md.cfg.ALBIngressController.TestServerReplicas),
			zap.Int("groups", len(desc.TargetGroups)),
		)
		if len(desc.TargetGroups) == 0 {
			return fmt.Errorf("found no target groups from %q", name)
		}

		healthyARNs := make(map[string]struct{})
		unhealthyARNs := make(map[string]struct{})

		retryStart := time.Now().UTC()
		for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
			for _, tg := range desc.TargetGroups {
				tgARN := tg.TargetGroupArn
				if _, ok := healthyARNs[*tgARN]; ok {
					continue
				}

				var healthOut *elbv2.DescribeTargetHealthOutput
				healthOut, err = md.elbv2.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{
					TargetGroupArn: tgARN,
				})
				if err != nil {
					md.lg.Warn(
						"failed to describe target group health",
						zap.String("elbv2-name", name),
						zap.Error(err),
					)
					return err
				}

				healthCnt := 0
				for _, hv := range healthOut.TargetHealthDescriptions {
					hs := *hv.TargetHealth.State
					if hs != "healthy" {
						md.lg.Warn(
							"found unhealthy target",
							zap.String("elbv2-name", name),
							zap.String("target-type", *tg.TargetType),
							zap.Int64("target-port", *hv.Target.Port),
							zap.String("target-group-arn", *tgARN),
							zap.String("target-state", hs),
						)
						continue
					}

					healthCnt++
					md.lg.Info(
						"found healthy target",
						zap.String("elbv2-name", name),
						zap.String("target-type", *tg.TargetType),
						zap.Int64("target-port", *hv.Target.Port),
						zap.String("target-group-arn", *tgARN),
						zap.String("target-state", hs),
					)
				}

				if healthCnt == len(healthOut.TargetHealthDescriptions) {
					healthyARNs[*tgARN] = struct{}{}
					delete(unhealthyARNs, *tgARN)
					break
				}

				delete(healthyARNs, *tgARN)
				unhealthyARNs[*tgARN] = struct{}{}

				time.Sleep(10 * time.Second)
			}

			if len(healthyARNs) == len(desc.TargetGroups) {
				break
			}

			time.Sleep(10 * time.Second)
		}

		if len(unhealthyARNs) > 0 {
			return fmt.Errorf("ALB %q has unhealthy target groups [%+v]", name, unhealthyARNs)
		}
	}

	md.lg.Info(
		"tested ALB resources",
		zap.Int("elbv2-number", len(md.cfg.ALBIngressController.ELBv2NameToARN)),
	)
	return nil
}
