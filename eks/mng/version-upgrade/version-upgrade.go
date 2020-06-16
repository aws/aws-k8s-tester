// Package versionupgrade implements EKS cluster version upgrade tester.
package versionupgrade

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
)

// Upgrader defines MNG version upgrade interface.
type Upgrader interface {
	// Upgrade starts MNG version upgrade process, and waits for its completion.
	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-version.html
	Upgrade(mngName string) error
}

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

// New creates a new Upgrader.
func New(cfg Config) Upgrader {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Upgrade(mngName string) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		ts.cfg.Logger.Warn("MNG not found; failing upgrade", zap.String("mng-name", mngName))
		return fmt.Errorf("MNGs[%q] not found; failed to upgrade", mngName)
	}
	if cur.VersionUpgrade == nil || !cur.VersionUpgrade.Enable {
		ts.cfg.Logger.Info("MNG version upgrade is not enabled; skipping upgrade", zap.String("mng-name", mngName))
		return nil
	}
	if cur.VersionUpgrade.Created {
		ts.cfg.Logger.Info("MNG version upgrade is already completed; skipping upgrade", zap.String("mng-name", mngName))
		return nil
	}
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]MNGs[%q].Upgrade\n"), mngName)
	fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))

	ts.cfg.Logger.Info("starting tester.Upgrade", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	cur.VersionUpgrade.Created = true
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	cur, _ = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		cur.VersionUpgrade.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("waiting before starting MNG upgrade",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.Duration("initial-wait", cur.VersionUpgrade.InitialWait),
	)
	select {
	case <-time.After(cur.VersionUpgrade.InitialWait):
		ts.cfg.Logger.Info("waited, starting MNG version upgrade",
			zap.String("cluster-name", ts.cfg.EKSConfig.Name),
			zap.String("mng-name", mngName),
			zap.Float64("cluster-version", ts.cfg.EKSConfig.Status.ServerVersionInfo.VersionValue),
			zap.String("target-mng-version", cur.VersionUpgrade.Version),
		)
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("MNG version upgrade wait aborted; exiting", zap.String("mng-name", mngName))
		return errors.New("MNG veresion upgrade wait aborted")
	}

	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-version.html
	var updateOut *eks.UpdateNodegroupVersionOutput
	updateOut, err = ts.cfg.EKSAPI.UpdateNodegroupVersion(&eks.UpdateNodegroupVersionInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(mngName),
		Version:       aws.String(cur.VersionUpgrade.Version),
	})
	if err != nil {
		ts.cfg.Logger.Warn("MNG version upgrade request failed", zap.String("mng-name", mngName), zap.Error(err))
		return err
	}
	reqID := ""
	if updateOut.Update != nil {
		reqID = aws.StringValue(updateOut.Update.Id)
	}

	// takes TODO
	initialWait := 3 * time.Minute
	totalWait := 5*time.Minute + 20*time.Second*time.Duration(cur.ASGDesiredCapacity)

	ts.cfg.Logger.Info("sent MNG upgrade request; polling",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.String("request-id", reqID),
		zap.Duration("total-wait", totalWait),
	)

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), totalWait)
	updateCh := Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		mngName,
		reqID,
		eks.UpdateStatusSuccessful,
		initialWait,
		30*time.Second,
	)
	for v := range updateCh {
		err = v.Error
	}
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), totalWait)
	nodesCh := wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		mngName,
		aws_eks.NodegroupStatusActive,
		initialWait,
		20*time.Second,
	)
	for sv := range nodesCh {
		status := ""
		if sv.NodeGroup != nil {
			status = aws.StringValue(sv.NodeGroup.Status)
		}
		if sv.Error != nil {
			cur.Status = fmt.Sprintf("%q failed upgrade with error %v", sv.NodeGroupName, sv.Error)
		} else {
			cur.Status = status
		}
		if sv.Error != nil && status == aws_eks.NodegroupStatusCreateFailed {
			ts.cfg.Logger.Warn("failed to upgrade managed node group",
				zap.String("status", status),
				zap.Error(sv.Error),
			)
			cancel()
			ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
			ts.cfg.EKSConfig.Sync()
			return sv.Error
		}
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		cur, _ = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
		ts.cfg.EKSConfig.Sync()
	}
	cancel()

	ts.cfg.Logger.Info("checking EKS server health after MNG version upgrade")
	waitDur, retryStart := 5*time.Minute, time.Now()
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
		ts.cfg.Logger.Warn("health check failed after MNG version upgrade", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("completed MNG version upgrade",
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	return nil
}
