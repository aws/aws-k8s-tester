package alb

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/awstester/pkg/fileutil"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) CreateRBAC() error {
	now := time.Now().UTC()

	p, err := fileutil.WriteTempFile([]byte(albYAMLRBAC))
	if err != nil {
		return err
	}
	defer os.RemoveAll(p)

	kcfgPath := md.cfg.KubeConfigPath
	var kexo []byte
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"apply", "--filename="+p,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to apply RBAC for ALB Ingress Controller",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)
			time.Sleep(5 * time.Second)
			continue
		}
		md.lg.Info("applied RBAC for ALB Ingress Controller", zap.String("output", string(kexo)))
		break
	}

	time.Sleep(3 * time.Second)

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"get", "clusterroles",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get pods", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		if strings.Contains(string(kexo), "alb-ingress-controller") {
			break
		}

		md.lg.Warn("creating RBAC for ALB Ingress Controller", zap.String("output", string(kexo)), zap.Error(err))
		time.Sleep(5 * time.Second)
		continue
	}
	if !strings.Contains(string(kexo), "alb-ingress-controller") {
		return errors.New("cannot get cluster role 'alb-ingress-controller'")
	}

	md.lg.Info(
		"created RBAC for ALB Ingress Controller",
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}
