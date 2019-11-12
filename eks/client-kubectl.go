package eks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html
// https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
// aws eks update-kubeconfig --name --role-arn --kubeconfig
func (ts *Tester) updateKUBECONFIG() error {
	args := []string{
		"eks",
		fmt.Sprintf("--region=%s", ts.cfg.Region),
		"update-kubeconfig",
		fmt.Sprintf("--name=%s", ts.cfg.Name),
		fmt.Sprintf("--kubeconfig=%s", ts.cfg.KubeConfigPath),
		"--verbose",
	}
	if ts.cfg.Parameters.ClusterResolverURL != "" {
		args = append(args, fmt.Sprintf("--endpoint=%s", ts.cfg.Parameters.ClusterResolverURL))
	}
	ts.lg.Info("writing KUBECONFIG with 'aws eks update-kubeconfig'",
		zap.String("kubeconfig-path", ts.cfg.KubeConfigPath),
		zap.String("aws-cli-path", ts.cfg.AWSCLIPath),
		zap.Strings("aws-args", args),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	ao, err := exec.New().CommandContext(
		ctx,
		ts.cfg.AWSCLIPath,
		args...,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'aws eks update-kubeconfig' failed (output %q, error %v)", string(ao), err)
	}
	ts.lg.Info("'aws eks update-kubeconfig' success", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
	return ts.cfg.Sync()
}

func (ts *Tester) pollClusterInfo(timeout, interval time.Duration) error {
	retryStart := time.Now().UTC()
	ticker := time.NewTicker(interval)
	for time.Now().UTC().Sub(retryStart) < timeout {
		select {
		case <-ticker.C:
		case <-ts.stopCreationCh:
			return nil
		}
		d, err := ts.getClusterInfo()
		if err == nil {
			println()
			fmt.Println("kubectl cluster-info:")
			fmt.Println(d)
			println()
			return nil
		}
		ts.lg.Warn("failed to get cluster info; retrying", zap.Error(err))
	}
	return nil
}

func (ts *Tester) getClusterInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	clusterInfoOut, err := exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"cluster-info",
	).CombinedOutput()
	cancel()
	out := string(clusterInfoOut)
	if err != nil {
		return "", fmt.Errorf("'kubectl cluster-info' failed (output %q, error %v)", out, err)
	}
	if !strings.Contains(out, "is running at") {
		return "", fmt.Errorf("'kubectl cluster-info' not ready (output %q)", out)
	}
	return out, nil
}
