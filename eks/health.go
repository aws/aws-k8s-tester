package eks

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (ts *Tester) checkHealth() (err error) {
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
	colorstring.Printf("\n\"[light_green]kubectl version[default]\" output:\n%s\n", out)

	ep := ts.cfg.Status.ClusterAPIServerEndpoint + "/version"
	buf := bytes.NewBuffer(nil)
	if err = httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, fmt.Sprintf(`"gitVersion": "v%s`, ts.cfg.Parameters.Version)) {
		return fmt.Errorf("%q does not contain version %q", out, ts.cfg.Parameters.Version)
	}
	colorstring.Printf("\n\n\"[light_green]%s[default]\" output:\n%s\n", ep, out)

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
	colorstring.Printf("\n\"[light_green]kubectl cluster-info[default]\" output:\n%s\n", out)

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
	colorstring.Printf("\n\"[light_green]kubectl get cs[default]\" output:\n%s\n", out)

	ep = ts.cfg.Status.ClusterAPIServerEndpoint + "/healthz?verbose"
	buf.Reset()
	if err := httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, "healthz check passed") {
		return fmt.Errorf("%q does not contain 'healthz check passed'", out)
	}
	colorstring.Printf("\n\n\"[light_green]%s[default]\" output:\n%s\n", ep, out)

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
	colorstring.Printf("\n\"[light_green]kubectl all -n=kube-system[default]\" output:\n%s\n", out)

	colorstring.Printf("\n\"[light_green]kubectl get pods -n=kube-system[default]\" output:\n")
	pods, err := ts.getPods("kube-system")
	if err != nil {
		return fmt.Errorf("failed to get pods %v", err)
	}
	for _, v := range pods.Items {
		colorstring.Printf("[light_magenta]kube-system Pod[default]: %q\n", v.Name)
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
	colorstring.Printf("\n\"[light_green]kubectl get configmaps --all-namespaces[default]\" output:\n%s\n", out)

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
	colorstring.Printf("\n\"[light_green]kubectl get namespaces[default]\" output:\n%s\n", out)

	colorstring.Printf("\n\"[light_green]curl -sL http://localhost:8080/metrics | grep storage_[default]\" output:\n")
	output, err = ts.k8sClientSet.
		CoreV1().
		RESTClient().
		Get().
		RequestURI("/metrics").
		Do().
		Raw()
	if err != nil {
		return fmt.Errorf("failed to fetch /metrics (%v)", err)
	}
	const (
		metricDEKGen            = "apiserver_storage_data_key_generation_latencies_microseconds_count"
		metricEnvelopeCacheMiss = "apiserver_storage_envelope_transformation_cache_misses_total"
	)
	dekGenCnt, cacheMissCnt := int64(0), int64(0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "# "):
			continue

		case strings.HasPrefix(line, metricDEKGen+" "):
			vs := strings.TrimSpace(strings.Replace(line, metricDEKGen, "", -1))
			dekGenCnt, err = strconv.ParseInt(vs, 10, 64)
			if err != nil {
				ts.lg.Warn("failed to parse",
					zap.String("line", line),
					zap.Error(err),
				)
			}

		case strings.HasPrefix(line, metricEnvelopeCacheMiss+" "):
			vs := strings.TrimSpace(strings.Replace(line, metricEnvelopeCacheMiss, "", -1))
			cacheMissCnt, err = strconv.ParseInt(vs, 10, 64)
			if err != nil {
				ts.lg.Warn("failed to parse",
					zap.String("line", line),
					zap.Error(err),
				)
			}
		}

		if dekGenCnt > 0 || cacheMissCnt > 0 {
			break
		}
	}
	ts.lg.Info("encryption metrics",
		zap.Int64("dek-gen-count", dekGenCnt),
		zap.Int64("cache-miss-count", cacheMissCnt),
	)
	if ts.cfg.Status.EncryptionCMKARN != "" && dekGenCnt <= 0 && cacheMissCnt <= 0 {
		ts.lg.Info("encryption is not enabled")
		// return errors.New("encrypted enabled, unexpected /metrics")
	}

	ts.lg.Info("checked /metrics")
	return nil
}
