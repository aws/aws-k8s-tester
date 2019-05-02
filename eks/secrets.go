package eks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// if this changes, make sure to update "internal/ingress" for volume mounts, as well
const awsCredentialSecretName = "aws-cred-aws-k8s-tester"

// TODO: use "k8s.io/client-go" with "aws-iam-authenticator"
func (md *embedded) createAWSCredentialSecret() error {
	if md.cfg.AWSCredentialToMountPath == "" {
		md.lg.Info("no AWS credentials to mount")
		return nil
	}
	if md.cfg.KubeConfigPath == "" {
		return errors.New("cannot find KUBECONFIG")
	}

	now := time.Now().UTC()

	md.lg.Info("kubectl create secret generic")
	var kexo []byte
	var err error
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		kexo, err = exec.New().CommandContext(ctx,
			md.cfg.KubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"create", "secret", "generic", awsCredentialSecretName,
			"--namespace=kube-system",
			fmt.Sprintf("--from-file=%s=%s", awsCredentialSecretName, md.cfg.AWSCredentialToMountPath),
		).CombinedOutput()
		cancel()
		if err != nil {
			if strings.Contains(err.Error(), "unknown flag:") {
				return fmt.Errorf("unknown flag %s", string(kexo))
			}
			md.lg.Warn("failed to create secret",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)
			time.Sleep(5 * time.Second)
			continue
		}
		md.lg.Info("created secret", zap.String("output", string(kexo)))
		break
	}

	time.Sleep(3 * time.Second)

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		kexo, err = exec.New().CommandContext(ctx,
			md.cfg.KubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "secret", awsCredentialSecretName,
			"--output=yaml",
			"--namespace=kube-system",
		).CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get secret", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		md.lg.Info("got secret")
		break
	}
	if !strings.Contains(string(kexo), fmt.Sprintf("name: %s", awsCredentialSecretName)) {
		return errors.New("cannot get secret objects")
	}

	md.lg.Info("kubectl created secret generic", zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")))
	return nil
}
