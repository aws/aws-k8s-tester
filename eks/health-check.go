package eks

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (ts *Tester) runHealthCheck() error {
	ts.lg.Info("running health check")

	if err := ts.listPods("kube-system"); err != nil {
		ts.lg.Warn("listing pods failed", zap.Error(err))
		ts.cfg.Status.ClusterStatus = fmt.Sprintf("listing pods failed (%v)", err)
		ts.cfg.Sync()
		return err
	}

	// might take several minutes for DNS to propagate
	waitDur := 5 * time.Minute
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitDur {
		select {
		case <-ts.stopCreationCh:
			return errors.New("health check aborted")
		case <-time.After(5 * time.Second):
		}
		err := ts.healthCheck()
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

func (ts *Tester) healthCheck() error {
	ep := ts.cfg.Status.ClusterAPIServerEndpoint + "/version"
	buf := bytes.NewBuffer(nil)
	if err := httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out := buf.String()
	if !strings.Contains(out, fmt.Sprintf(`"gitVersion": "v%s`, ts.cfg.Parameters.Version)) {
		return fmt.Errorf("%q does not contain version %q", out, ts.cfg.Parameters.Version)
	}
	fmt.Printf("\n%q:\n%s\n\n", ep, out)

	ep = ts.cfg.Status.ClusterAPIServerEndpoint + "/healthz?verbose"
	buf.Reset()
	if err := httpReadInsecure(ts.lg, ep, buf); err != nil {
		return err
	}
	out = buf.String()
	if !strings.Contains(out, "healthz check passed") {
		return fmt.Errorf("%q does not contain 'healthz check passed'", out)
	}
	fmt.Printf("\n%q:\n%s\n\n", ep, out)

	return ts.cfg.Sync()
}
