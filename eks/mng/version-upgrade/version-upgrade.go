// Package versionupgrade implements EKS cluster version upgrade tester.
package versionupgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/spinner"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
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
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Upgrader.
func New(cfg Config) Upgrader {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
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
	fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]MNGs[%q].Upgrade\n"), mngName)

	ts.cfg.Logger.Info("starting tester.Upgrade", zap.String("tester", pkgName))
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

	sp := spinner.New(ts.cfg.LogWriter, "Waiting for before starting MNG upgrade "+mngName)
	ts.cfg.Logger.Info("waiting before starting MNG upgrade",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.Duration("initial-wait", cur.VersionUpgrade.InitialWait),
	)
	sp.Restart()
	select {
	case <-time.After(cur.VersionUpgrade.InitialWait):
		sp.Stop()
		ts.cfg.Logger.Info("waited, starting MNG version upgrade",
			zap.String("cluster-name", ts.cfg.EKSConfig.Name),
			zap.String("mng-name", mngName),
			zap.Float64("cluster-version", ts.cfg.EKSConfig.Status.ServerVersionInfo.VersionValue),
			zap.String("target-mng-version", cur.VersionUpgrade.Version),
		)
	case <-ts.cfg.Stopc:
		sp.Stop()
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

	initialWait := 5 * time.Minute
	checkN := time.Duration(cur.ASGDesiredCapacity)
	if checkN == 0 {
		checkN = time.Duration(cur.ASGMinSize)
	}
	totalWait := 2*time.Hour + 30*time.Minute + 3*time.Minute*checkN
	ts.cfg.Logger.Info("sent MNG upgrade request; polling",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.String("request-id", reqID),
		zap.Int("asg-desired-capacity", cur.ASGDesiredCapacity),
		zap.Duration("total-wait", totalWait),
	)

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), totalWait)
	updateCh := wait.PollUpdate(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		mngName,
		reqID,
		eks.UpdateStatusSuccessful,
		initialWait,
		2*time.Minute,
		wait.WithQueryFunc(func() {
			fmt.Fprintf(ts.cfg.LogWriter, "\n")
			ts.cfg.Logger.Info("listing nodes while polling mng update status", zap.String("mng-name", mngName))
			nodes, err := ts.cfg.K8SClient.ListNodes(1000, 5*time.Second)
			if err != nil {
				ts.cfg.Logger.Warn("failed to list nodes while polling mng update status", zap.Error(err))
				return
			}
			cnt := 0
			for _, node := range nodes {
				labels := node.GetLabels()
				if labels["NGName"] != mngName {
					continue
				}
				cnt++
				for _, cond := range node.Status.Conditions {
					if cond.Status != v1.ConditionTrue {
						continue
					}
					ts.cfg.Logger.Info("node",
						zap.String("name", node.GetName()),
						zap.String("mng-name", mngName),
						zap.String("status-type", fmt.Sprintf("%s", cond.Type)),
						zap.String("status", fmt.Sprintf("%s", cond.Status)),
					)
					break
				}
			}
			ts.cfg.Logger.Info("listed nodes while polling mng update status", zap.String("mng-name", mngName), zap.Int("total-nodes", cnt))
		}),
	)
	for v := range updateCh {
		err = v.Error
	}
	cancel()
	if err != nil {
		cur.Status = fmt.Sprintf("update failed %v", err)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
		return fmt.Errorf("MNGs[%q] update failed %v", mngName, err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), totalWait)
	nodesCh := wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		mngName,
		aws_eks.NodegroupStatusActive,
		initialWait,
		20*time.Second,
	)
	for sv := range nodesCh {
		err = sv.Error
	}
	cancel()
	if err != nil {
		cur.Status = fmt.Sprintf("update failed %v", err)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
		return fmt.Errorf("MNGs[%q] update failed %v", mngName, err)
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}
