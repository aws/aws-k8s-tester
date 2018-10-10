package alb

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/awstester/internal/eks/ingress"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) DeployIngressController() error {
	now := time.Now().UTC()

	// TODO: git pull from PR and build test image
	// and push to ECR with AWS account ID + AWS region
	cfg := ingress.ConfigDeploymentServiceALBIngressController{
		AWSRegion:   md.cfg.AWSRegion,
		Name:        "alb-ingress-controller",
		ServiceName: "alb-ingress-controller-service",
		Namespace:   "kube-system",
		Image:       md.cfg.ALBIngressController.ALBIngressControllerImage,
		ClusterName: md.cfg.ClusterName,
	}
	d, err := ingress.CreateDeploymentServiceALBIngressController(cfg)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(md.cfg.ALBIngressController.IngressControllerSpecPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(d))
	if err != nil {
		return err
	}
	f.Close()

	kcfgPath := md.cfg.KubeConfigPath
	md.lg.Info("kubectl apply alb-ingress-controller")

	md.cfg.ALBIngressController.DeploymentStatus = "CREATING"
	md.cfg.Sync()

	// usually takes 3-minute
	md.lg.Info("waiting for 2-minute")
	time.Sleep(2 * time.Minute)

	var kexo []byte

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"apply",
			"--filename="+md.cfg.ALBIngressController.IngressControllerSpecPath,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to apply alb-ingress-controller deployment and service",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)
			md.cfg.ALBIngressController.DeploymentStatus = err.Error()
			md.cfg.Sync()

			md.lg.Info("creating ingress controller")
			time.Sleep(5 * time.Second)
			continue
		}

		md.cfg.ALBIngressController.DeploymentStatus = "APPLIED"
		md.cfg.Sync()

		md.lg.Info("applied", zap.String("output", string(kexo)))
		break
	}

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "pods", "--output=yaml",
			"--namespace="+cfg.Namespace,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get pods", zap.Error(err))
			md.cfg.ALBIngressController.DeploymentStatus = err.Error()
			md.cfg.Sync()
			time.Sleep(5 * time.Second)
			continue
		}
		if findReadyPodsFromKubectlGetPodsOutputYAML(kexo, cfg.Name) {
			md.lg.Info("pod is ready")
			md.cfg.ALBIngressController.DeploymentStatus = "READY"
			md.cfg.Sync()
			break
		}

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "pods",
			"--namespace="+cfg.Namespace,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get pods", zap.String("output", string(kexo)), zap.Error(err))
			md.cfg.ALBIngressController.DeploymentStatus = err.Error()
			md.cfg.Sync()
			time.Sleep(5 * time.Second)
			continue
		}

		md.lg.Info("creating ingress controller", zap.String("output", string(kexo)), zap.Error(err))
		md.cfg.ALBIngressController.DeploymentStatus = err.Error()
		md.cfg.Sync()
		time.Sleep(5 * time.Second)
		continue
	}
	if !strings.Contains(string(kexo), cfg.Name) {
		return errors.New("cannot get pod objects")
	}

	md.lg.Info(
		"created alb-ingress-controller deployment and service",
		zap.String("name", cfg.Name),
		zap.String("namespace", cfg.Namespace),
		zap.String("service-name", cfg.ServiceName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}
