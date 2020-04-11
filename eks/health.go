package eks

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/eks"
	"go.uber.org/zap"
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
		ts.cfg.Status.ServerVersionInfo, err = ts.k8sClient.FetchServerVersion()
		if err != nil {
			ts.lg.Warn("get version failed", zap.Error(err))
		}
		err = ts.k8sClient.CheckHealth()
		if err == nil {
			break
		}
		ts.lg.Warn("health check failed", zap.Error(err))
		ts.cfg.RecordStatus(fmt.Sprintf("health check failed (%v)", err))
	}

	ts.lg.Info("health check success")
	return err
}
