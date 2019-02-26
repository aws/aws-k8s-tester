package alb

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress"
	"github.com/aws/aws-k8s-tester/internal/eks/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Plugin defines ALB Ingress Controller deployer operations.
type Plugin interface {
	DeployBackend() error

	CreateRBAC() error

	DeployIngressController() error

	CreateSecurityGroup() error
	DeleteSecurityGroup() error

	CreateIngressObjects() error
	DeleteIngressObjects() error

	TestAWSResources() error
}

type embedded struct {
	stopc chan struct{}

	lg  *zap.Logger
	cfg *eksconfig.Config

	// TODO: move this "kubectl" to AWS CLI deployer
	// and instead use "k8s.io/client-go" with STS token
	kubectl     exec.Interface
	kubectlPath string

	im       iamiface.IAMAPI
	ec2      ec2iface.EC2API
	elbv2    elbv2iface.ELBV2API
	s3Plugin s3.Plugin
}

// NewEmbedded creates a new Plugin using AWS CLI.
func NewEmbedded(
	stopc chan struct{},
	lg *zap.Logger,
	cfg *eksconfig.Config,
	kubectlPath string,
	im iamiface.IAMAPI,
	ec2 ec2iface.EC2API,
	elbv2 elbv2iface.ELBV2API,
	s3Plugin s3.Plugin,
) Plugin {
	return &embedded{
		stopc:       stopc,
		lg:          lg,
		cfg:         cfg,
		kubectlPath: kubectlPath,
		kubectl:     exec.New(),
		im:          im,
		ec2:         ec2,
		elbv2:       elbv2,
		s3Plugin:    s3Plugin,
	}
}

func (md *embedded) DeployBackend() error {
	now := time.Now().UTC()

	// create config map for nginx config and response body
	err := md.createNginxConfigMap()
	if err != nil {
		return err
	}
	cfg := ingress.ConfigNginx{
		Namespace: "default",
		Replicas:  md.cfg.ALBIngressController.TestServerReplicas,
	}
	name := "nginx-deployment"
	d, err := ingress.CreateDeploymentServiceNginx(cfg)
	if err != nil {
		return err
	}

	var f *os.File
	f, err = os.OpenFile(
		md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPath,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0600,
	)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(d))
	if err != nil {
		return err
	}
	f.Close()

	kcfgPath := md.cfg.KubeConfigPath

	// usually takes 1-minute
	md.lg.Info("waiting for 1-minute")
	time.Sleep(time.Minute)

	var kexo []byte
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"apply",
			"--filename="+md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPath,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to apply deployment and service",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)
			time.Sleep(5 * time.Second)
			continue
		}
		md.lg.Info("applied ingress test server", zap.String("output", string(kexo)))
		break
	}

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "pods", "--output=yaml",
			"--namespace="+"default",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get pods", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		if findReadyPodsFromKubectlGetPodsOutputYAML(kexo, name) {
			md.lg.Info("ingress test server deployment is ready", zap.String("name", name))
			break
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "pods",
			"--namespace="+"default",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get pods", zap.String("output", string(kexo)), zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		md.lg.Warn("creating ingress test server", zap.String("output", string(kexo)), zap.Error(err))
		time.Sleep(5 * time.Second)
		continue
	}
	if !strings.Contains(string(kexo), name) {
		return errors.New("cannot get pod objects")
	}

	md.lg.Info(
		"created ingress test server",
		zap.String("name", name),
		zap.String("namespace", "default"),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}

func (md *embedded) createNginxConfigMap() error {
	d, err := ingress.CreateConfigMapNginx(md.cfg.ALBIngressController.TestResponseSize)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPath, []byte(d), 0600)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := md.kubectl.CommandContext(ctx,
		md.kubectlPath,
		"--kubeconfig="+md.cfg.KubeConfigPath,
		"apply",
		"--filename="+md.cfg.ALBIngressController.IngressTestServerDeploymentServiceSpecPath,
	)
	kexo, err := cmd.CombinedOutput()
	cancel()
	if err != nil {
		return err
	}
	md.lg.Info("applied nginx config map", zap.String("output", string(kexo)))
	return nil
}

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

						time.Sleep(10 * time.Second)
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
