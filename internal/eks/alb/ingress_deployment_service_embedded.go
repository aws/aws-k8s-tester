package alb

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/aws/awstester/internal/eks/alb/ingress"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

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

func (md *embedded) DeployBackend() error {
	now := time.Now().UTC()

	var name string
	var d string
	var err error
	switch md.cfg.ALBIngressController.TestMode {
	case "ingress-test-server":
		cfg := ingress.ConfigDeploymentServiceIngressTestServer{
			Name:         "ingress-test-server",
			ServiceName:  "ingress-test-server-service",
			Namespace:    "default",
			Image:        md.cfg.AWSTesterImage,
			Replicas:     md.cfg.ALBIngressController.TestServerReplicas,
			Routes:       md.cfg.ALBIngressController.TestServerRoutes,
			ResponseSize: md.cfg.ALBIngressController.TestResponseSize,
		}
		d, err = ingress.CreateDeploymentServiceIngressTestServer(cfg)
		name = cfg.Name

	case "nginx":
		// create config map for nginx config and response body
		if err = md.createNginxConfigMap(); err != nil {
			return err
		}
		cfg := ingress.ConfigNginx{
			Namespace: "default",
			Replicas:  md.cfg.ALBIngressController.TestServerReplicas,
		}
		d, err = ingress.CreateDeploymentServiceNginx(cfg)
		name = "nginx-deployment"

	default:
		return fmt.Errorf("%q is unknown", md.cfg.ALBIngressController.TestMode)
	}
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
