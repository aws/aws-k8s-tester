// Package configupdate implements EKS cluster config update tester.
package configupdate

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	version_upgrade "github.com/aws/aws-k8s-tester/eks/mng/version-upgrade"
	"github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
)

// Updater defines MNG config update interface.
type Updater interface {
	// Update starts MNG config update process, and waits for its completion.
	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
	Update(mngName string) error
}

// Config defines config update configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

// New creates a new Updater.
func New(cfg Config) Updater {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

var reqID = ""

func (ts *tester) Update(mngName string) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		ts.cfg.Logger.Warn("MNG not found; failing update", zap.String("mng-name", mngName))
		return fmt.Errorf("MNGs[%q] not found; failed to update", mngName)
	}
	if cur.ConfigUpdates[0] == nil {
		ts.cfg.Logger.Info("MNG config update is not enabled; skipping update", zap.String("mng-name", mngName))
		return nil
	}
	index := 0
	for i:=0; i < len(cur.ConfigUpdates); i++ {
		if cur.ConfigUpdates[index].Created {
			index++;
			if i == len(cur.ConfigUpdates) - 1 {
				ts.cfg.Logger.Info("All updates have been applied for this MNG", zap.String("mng-name", mngName))
				return nil
			}
		} else {
			break
		}
	}

	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************[default]\n")
		colorstring.Printf("[light_green]MNGs[%q].Update[default]\n", mngName)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("MNGs[%q].Update\n", mngName)
	}

	ts.cfg.Logger.Info("starting tester.Update", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	cur.ConfigUpdates[index].Created = true
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	cur, _ = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		cur.ConfigUpdates[index].TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("waiting before starting MNG update",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.Duration("initial-wait", cur.ConfigUpdates[index].InitialWait),
	)
	select {
	case <-time.After(cur.ConfigUpdates[index].InitialWait):
		ts.cfg.Logger.Info("waited, starting MNG config update",
			zap.String("cluster-name", ts.cfg.EKSConfig.Name),
			zap.String("mng-name", mngName),
			zap.Int("asg-min-size", cur.ASGMinSize),
			zap.Int("asg-max-size", cur.ASGMaxSize),
			zap.Int("asg-desired-capacity", cur.ASGDesiredCapacity),
			zap.Int64("target-min-size", cur.ConfigUpdates[index].MinSize),
			zap.Int64("target-max-size", cur.ConfigUpdates[index].MaxSize),
			zap.Int64("target-desired-size", cur.ConfigUpdates[index].DesiredSize),
		)
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("MNG config update wait aborted; exiting", zap.String("mng-name", mngName))
		return errors.New("MNG config update wait aborted")
	}

	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
	var scaleErr = ts.scaleMNG(mngName, index)
	if scaleErr != nil {
		return scaleErr
	}
	
	// takes TODO
	initialWait := 3 * time.Minute
	totalWait := 5*time.Minute + 20*time.Second*time.Duration(cur.ASGDesiredCapacity)

	ts.cfg.Logger.Info("sent MNG config update request; polling",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.String("request-id", reqID),
		zap.Duration("total-wait", totalWait),
	)

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), totalWait)
	updateCh := version_upgrade.Poll(
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
			cur.Status = fmt.Sprintf("%q failed update with error %v", sv.NodeGroupName, sv.Error)
		} else {
			cur.Status = status
		}
		if sv.Error != nil && status == aws_eks.NodegroupStatusCreateFailed {
			ts.cfg.Logger.Warn("failed to update managed node group",
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

	ts.cfg.Logger.Info("checking EKS server health after MNG config update")
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
		ts.cfg.Logger.Warn("health check failed after MNG config update", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("completed MNG config update",
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	return nil
}

// Scales MNG to the next scaling configuration in the array.
func (ts *tester) scaleMNG(mngName string, index int) (err error) {
	cur := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] 
	nodegroupConfigInput := &aws_eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(mngName),
		ScalingConfig: &aws_eks.NodegroupScalingConfig{DesiredSize: aws.Int64(cur.ConfigUpdates[index].DesiredSize),
			MaxSize: aws.Int64(cur.ConfigUpdates[index].MaxSize),
			MinSize: aws.Int64(cur.ConfigUpdates[index].MinSize)}}
	var updateOut *eks.UpdateNodegroupConfigOutput
	updateOut, err = ts.cfg.EKSAPI.UpdateNodegroupConfig(nodegroupConfigInput)

	if err != nil {
		ts.cfg.Logger.Warn("MNG config update request failed", zap.String("mng-name", mngName), zap.Error(err))
		return err
	}
	if updateOut.Update != nil {
		reqID = aws.StringValue(updateOut.Update.Id)
	}
	return nil
}
