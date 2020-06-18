// Package versionupgrade implements EKS cluster version upgrade tester.
package versionupgrade

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/aws/aws-k8s-tester/eks/cluster/wait"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
)

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("starting cluster version upgrade",
		zap.String("name", ts.cfg.EKSConfig.Name),
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	var updateOut *eks.UpdateClusterVersionOutput
	updateOut, err = ts.cfg.EKSAPI.UpdateClusterVersion(&eks.UpdateClusterVersionInput{
		Name:    aws.String(ts.cfg.EKSConfig.Name),
		Version: aws.String(ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	})
	if err != nil {
		ts.cfg.Logger.Warn("cluster version upgrade request failed", zap.String("name", ts.cfg.EKSConfig.Name), zap.Error(err))
		return err
	}
	reqID := ""
	if updateOut.Update != nil {
		reqID = aws.StringValue(updateOut.Update.Id)
	}
	ts.cfg.Logger.Info("sent upgrade cluster request",
		zap.String("name", ts.cfg.EKSConfig.Name),
		zap.String("request-id", reqID),
	)

	// takes ~30-min
	initialWait := 10 * time.Minute

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour+30*time.Minute)
	ch := wait.PollUpdate(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		reqID,
		eks.UpdateStatusSuccessful,
		initialWait,
		30*time.Second,
	)
	for sv := range ch {
		err = sv.Error
	}
	cancel()
	if err != nil {
		return fmt.Errorf("Cluster %q update failed %v", ts.cfg.EKSConfig.Name, err)
	}

	// may take a while to shut down the last master instance with old cluster version
	ts.cfg.Logger.Info("checking EKS server version after cluster version upgrade", zap.String("target-version", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version))
	waitDur, retryStart := 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("version check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}

		ts.cfg.EKSConfig.Status.ServerVersionInfo, err = ts.cfg.K8SClient.FetchServerVersion()
		if err != nil {
			ts.cfg.Logger.Warn("failed to fetch server version", zap.Error(err))
			continue
		}

		ts.cfg.EKSConfig.Sync()
		cur := fmt.Sprintf("%.2f", ts.cfg.EKSConfig.Status.ServerVersionInfo.VersionValue)
		target := fmt.Sprintf("%.2f", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.VersionValue)

		ts.cfg.Logger.Info("comparing version", zap.String("current", cur), zap.String("target", target))
		if cur != target {
			err = fmt.Errorf("EKS server version after upgrade expected %q, got %q [%+v]", target, cur, ts.cfg.EKSConfig.Status.ServerVersionInfo)
			ts.cfg.Logger.Warn("version mismatch; retrying")
			continue
		}

		err = nil
		ts.cfg.Logger.Info("version match success!")
		break
	}
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("checking EKS server health after cluster version upgrade")
	waitDur, retryStart = 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("health check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}
		err = ts.cfg.K8SClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
	}
	if err != nil {
		ts.cfg.Logger.Warn("health check failed after cluster version upgrade", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("completed cluster version upgrade",
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return nil
}
