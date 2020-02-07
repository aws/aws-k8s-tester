package eks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (ts *Tester) checkHealth() error {
	ts.lg.Info("running health check")

	if err := ts.listPods("kube-system"); err != nil {
		ts.lg.Warn("listing pods failed", zap.Error(err))
		ts.cfg.Status.ClusterStatus = fmt.Sprintf("listing pods failed (%v)", err)
		ts.cfg.Sync()
		return err
	}

	// might take several minutes for DNS to propagate
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.stopCreationCh:
			return errors.New("health check aborted")
		case <-ts.interruptSig:
			return errors.New("health check aborted")
		case <-time.After(5 * time.Second):
		}
		err := ts.health()
		if err == nil {
			break
		}
		ts.lg.Warn("health check failed", zap.Error(err))
		ts.cfg.Status.ClusterStatus = fmt.Sprintf("health check failed (%v)", err)
		ts.cfg.Sync()
	}

	ts.lg.Info("successfully ran health check")
	return ts.cfg.Sync()
}

func (ts *Tester) health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"version",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl version' failed %v", err)
	}
	out := string(output)
	colorstring.Printf("\n\n\"[light_green]kubectl version[default]\" output:\n%s\n\n", out)

	ep := ts.cfg.Status.ClusterAPIServerEndpoint + "/version"
	buf := bytes.NewBuffer(nil)
	if err = httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, fmt.Sprintf(`"gitVersion": "v%s`, ts.cfg.Parameters.Version)) {
		return fmt.Errorf("%q does not contain version %q", out, ts.cfg.Parameters.Version)
	}
	colorstring.Printf("\n\n[light_green]%s [default]output:\n%s\n\n", ep, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"cluster-info",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl cluster-info' failed %v", err)
	}
	out = string(output)
	if !strings.Contains(out, "is running at") {
		return fmt.Errorf("'kubectl cluster-info' not ready (output %q)", out)
	}
	colorstring.Printf("\n\n\"[light_green]kubectl cluster-info[default]\" output:\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"get",
		"cs",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl get cs' failed %v", err)
	}
	out = string(output)
	colorstring.Printf("\n\n\"[light_green]kubectl get cs[default]\" output:\n%s\n\n", out)

	ep = ts.cfg.Status.ClusterAPIServerEndpoint + "/healthz?verbose"
	buf.Reset()
	if err := httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, "healthz check passed") {
		return fmt.Errorf("%q does not contain 'healthz check passed'", out)
	}
	colorstring.Printf("\n\n[light_green]%s [default]output:\n%s\n\n", ep, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"--namespace=kube-system",
		"get",
		"all",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl get all -n=kube-system' failed %v", err)
	}
	out = string(output)
	colorstring.Printf("\n\n\"[light_green]kubectl get all -n=kube-system[default]\" output:\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"get",
		"configmaps",
		"--all-namespaces",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl get configmaps --all-namespaces' failed %v", err)
	}
	out = string(output)
	colorstring.Printf("\n\n\"[light_green]kubectl get configmaps --all-namespaces[default]\" output:\n%s\n\n", out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"get",
		"namespaces",
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl get namespaces' failed %v", err)
	}
	out = string(output)
	colorstring.Printf("\n\n\"[light_green]kubectl get namespaces[default]\" output:\n%s\n\n", out)

	return ts.cfg.Sync()
}
