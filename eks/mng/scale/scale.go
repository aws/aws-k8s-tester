// Package scale implements EKS cluster scaler tester.
package scale

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	wait "github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
)

// Scaler defines MNG scaler interface.
type Scaler interface {
	// Update starts MNG scaler process, and waits for its completion.
	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
	Scale(mngName string, update *eksconfig.MNGScaleUpdate) error
}

// Config defines scaler configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

// New creates a new Scaler.
func New(cfg Config) Scaler {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Scale(mngName string, update *eksconfig.MNGScaleUpdate) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		ts.cfg.Logger.Warn("MNG not found; failing update", zap.String("mng-name", mngName))
		return fmt.Errorf("MNGs[%q] not found; failed to update", mngName)
	}
	if len(cur.ScaleUpdates) == 0 {
		ts.cfg.Logger.Info("MNG scaler is not enabled; skipping update", zap.String("mng-name", mngName))
		return nil
	}
	for i, scaleUpdate := range cur.ScaleUpdates {
		if scaleUpdate.Created {
			if i == len(cur.ScaleUpdates)-1 {
				ts.cfg.Logger.Info("All updates have been applied for this MNG", zap.String("mng-name", mngName))
				return nil
			}
		}
	}
	fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]MNGs[%q].Scale\n"), mngName)

	ts.cfg.Logger.Info("starting tester.Update", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	update.Created = true
	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	cur, _ = ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		update.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("waiting before starting MNG update",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.Duration("initial-wait", update.InitialWait),
	)
	select {
	case <-time.After(update.InitialWait):
		ts.cfg.Logger.Info("waited, starting MNG scaler",
			zap.String("cluster-name", ts.cfg.EKSConfig.Name),
			zap.String("mng-name", mngName),
			zap.Int("asg-min-size", cur.ASGMinSize),
			zap.Int("asg-max-size", cur.ASGMaxSize),
			zap.Int("asg-desired-capacity", cur.ASGDesiredCapacity),
			zap.Int64("target-min-size", update.MinSize),
			zap.Int64("target-max-size", update.MaxSize),
			zap.Int64("target-desired-size", update.DesiredSize),
		)
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("MNG scaler wait aborted; exiting", zap.String("mng-name", mngName))
		return errors.New("MNG scaler wait aborted")
	}

	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
	if err = ts.scaleMNG(mngName, update); err != nil {
		return err
	}

	ts.cfg.Logger.Info("completed MNG scaler",
		zap.Int("from", cur.ASGDesiredCapacity),
		zap.Int64("to", update.DesiredSize),
	)
	return nil
}

// Scales MNG to the next scaling configuration in the array.
func (ts *tester) scaleMNG(mngName string, update *eksconfig.MNGScaleUpdate) (err error) {
	cur := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	nodegroupConfigInput := &aws_eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(mngName),
		ScalingConfig: &aws_eks.NodegroupScalingConfig{DesiredSize: aws.Int64(update.DesiredSize),
			MaxSize: aws.Int64(update.MaxSize),
			MinSize: aws.Int64(update.MinSize)}}
	var updateOut *eks.UpdateNodegroupConfigOutput
	updateOut, err = ts.cfg.EKSAPI.UpdateNodegroupConfig(nodegroupConfigInput)

	var reqID string
	if err != nil {
		ts.cfg.Logger.Warn("MNG scaler request failed", zap.String("mng-name", mngName), zap.Error(err))
		return err
	}
	if updateOut.Update != nil {
		reqID = aws.StringValue(updateOut.Update.Id)
	}

	// takes TODO
	initialWait := 3 * time.Minute
	totalWait := 5*time.Minute + 20*time.Second*time.Duration(cur.ASGDesiredCapacity)

	ts.cfg.Logger.Info("sent MNG scaler request; polling",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.String("request-id", reqID),
		zap.Duration("total-wait", totalWait),
	)

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), totalWait)
	updateCh := wait.PollUpdate(
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
		ts.cfg.Logger.Warn("MNG scale failed when polling", zap.String("mng-name", mngName), zap.Error(err))
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
		err = sv.Error
	}
	cancel()
	if err != nil {
		cur.Status = fmt.Sprintf("scale failed %v", err)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
		return fmt.Errorf("MNGs[%q] scale failed %v", mngName, err)
	}

	ts.cfg.Logger.Info("checking EKS server health after MNG scaler")
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
		ts.cfg.Logger.Warn("health check failed after MNG scaler", zap.Error(err))
		return err
	}
	return nil
}
