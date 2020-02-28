package eks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (ts *Tester) checkHealth() (err error) {
	colorstring.Printf("\n\n\"[light_green]checking health[default]\"\n\n")
	defer func() {
		if err == nil {
			ts.cfg.RecordStatus(eks.ClusterStatusActive)
		}
	}()

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
		err = ts.health()
		if err == nil {
			break
		}
		colorstring.Printf("\n\n\"[red]health check fail[default]\"\n\n")
		ts.lg.Warn("health check failed", zap.Error(err))
		ts.cfg.RecordStatus(fmt.Sprintf("health check failed (%v)", err))
	}

	colorstring.Printf("\n\n\"[light_green]health check success[default]\"\n\n")
	return err
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
	colorstring.Printf("\n\"[light_green]kubectl cluster-info[default]\" output:\n%s\n\n", out)

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
	colorstring.Printf("\n\"[light_green]kubectl get cs[default]\" output:\n%s\n\n", out)

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

	colorstring.Printf("\n\"[light_green]kubectl get pods -n=kube-system[default]\" output:\n")
	pods, err := ts.getPods("kube-system")
	if err != nil {
		return fmt.Errorf("failed to get pods %v", err)
	}
	for _, v := range pods.Items {
		colorstring.Printf("\"[light_magenta]kubectl get pods -n=kube-system[default]\" output: %q\n", v.Name)
	}
	println()

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
	colorstring.Printf("\n\"[light_green]kubectl get configmaps --all-namespaces[default]\" output:\n%s\n\n", out)

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
	colorstring.Printf("\n\"[light_green]kubectl get namespaces[default]\" output:\n%s\n\n", out)

	colorstring.Printf("\n\"[light_green]curl -sL http://localhost:8080/metrics | grep storage_[default]\" output:\n")
	mout, mfs, err := ts.metricsTester.Fetch()
	if err != nil {
		return fmt.Errorf("'/metrics' fetch failed %v", err)
	}
	found := false
	for _, k := range []string{metricDEKGen, metricEnvelopeCacheMiss} {
		mv, ok := mfs[k]
		if !ok {
			ts.lg.Warn("metric not found", zap.String("name", k))
			continue
		}
		found = true
		val := mv.Metric[0].GetCounter().GetValue()
		colorstring.Printf("\"[light_green]%s[default]\" metric output: %f\n", k, val)
	}
	if !found {
		colorstring.Printf("\"[red]/metrics not found[default]\" output:\n\n%s\n\n", string(mout))
		return errors.New("metrics not found")
	}
	ts.lg.Info("checked /metrics")
	return err
}

const (
	metricDEKGen            = "apiserver_storage_data_key_generation_latencies_microseconds_count"
	metricEnvelopeCacheMiss = "apiserver_storage_envelope_transformation_cache_misses_total"
)
