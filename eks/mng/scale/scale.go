// Package scale implements EKS cluster scaler tester.
// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
package scale

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	wait "github.com/aws/aws-k8s-tester/eks/mng/wait"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
)

// Scaler defines MNG scaler interface.
// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
type Scaler interface {
	// Update starts MNG scaler process, and waits for its completion.
	// ref. https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html
	Scale(mngName string) error
}

// Config defines scaler configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
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

func (ts *tester) Scale(mngName string) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		ts.cfg.Logger.Warn("MNG not found; failing update", zap.String("mng-name", mngName))
		return fmt.Errorf("MNGs[%q] not found; failed to update", mngName)
	}
	if len(cur.ScaleUpdates) == 0 {
		ts.cfg.Logger.Info("MNG scaler is not enabled; skipping update", zap.String("mng-name", mngName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Update", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	for idx, update := range cur.ScaleUpdates {
		if !update.Enable {
			continue
		}
		if update.Created {
			ts.cfg.Logger.Info("scale update already created; skipping", zap.Int("index", idx))
			continue
		}
		fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
		fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]MNGs[%q].Scale[%d]\n"), mngName, idx)
		ts.cfg.Logger.Info("waiting before starting MNG update",
			zap.String("cluster-name", ts.cfg.EKSConfig.Name),
			zap.String("mng-name", mngName),
			zap.Int("asg-min-size", cur.ASGMinSize),
			zap.Int("asg-max-size", cur.ASGMaxSize),
			zap.Int("asg-desired-capacity", cur.ASGDesiredCapacity),
			zap.Int64("target-min-size", update.ASGMinSize),
			zap.Int64("target-max-size", update.ASGMaxSize),
			zap.Int64("target-desired-size", update.ASGDesiredCapacity),
			zap.String("update-id", update.ID),
			zap.Duration("initial-wait", update.InitialWait),
		)
		select {
		case <-time.After(update.InitialWait):
			ts.cfg.Logger.Info("waited, starting MNG scaler")
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("MNG scaler wait aborted; exiting", zap.String("mng-name", mngName))
			return errors.New("MNG scaler wait aborted")
		}

		createStart := time.Now()
		if err = ts.scaleMNG(mngName, update); err != nil {
			return err
		}
		createEnd := time.Now()
		update.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		cur.ScaleUpdates[idx] = update

		ts.cfg.Logger.Info("completed MNG scaler",
			zap.Int("from", cur.ASGDesiredCapacity),
			zap.Int64("to", update.ASGDesiredCapacity),
		)
	}

	ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) scaleMNG(mngName string, update eksconfig.MNGScaleUpdate) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found", mngName)
	}

	var out *eks.UpdateNodegroupConfigOutput
	out, err = ts.cfg.EKSAPI.UpdateNodegroupConfig(&aws_eks.UpdateNodegroupConfigInput{
		ClusterName:   aws.String(ts.cfg.EKSConfig.Name),
		NodegroupName: aws.String(mngName),
		ScalingConfig: &aws_eks.NodegroupScalingConfig{
			DesiredSize: aws.Int64(update.ASGDesiredCapacity),
			MaxSize:     aws.Int64(update.ASGMaxSize),
			MinSize:     aws.Int64(update.ASGMinSize),
		},
	})
	if err != nil {
		ts.cfg.Logger.Warn("MNG scaler request failed", zap.String("mng-name", mngName), zap.Error(err))
		return err
	}

	reqID := ""
	if out.Update != nil {
		reqID = aws.StringValue(out.Update.Id)
	}
	if reqID == "" {
		return fmt.Errorf("MNGs[%q] UpdateNodegroupConfigOutput.Update.Id empty", mngName)
	}

	initialWait := 3 * time.Minute
	totalWait := time.Hour + 10*time.Minute*time.Duration(update.ASGDesiredCapacity)
	ts.cfg.Logger.Info("sent MNG scaler request; polling",
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
		zap.String("mng-name", mngName),
		zap.Int("current-asg-min-size", cur.ASGMinSize),
		zap.Int("current-asg-desired-capacity", cur.ASGDesiredCapacity),
		zap.Int("current-asg-max-size", cur.ASGMaxSize),
		zap.Int64("target-asg-min-size", update.ASGMinSize),
		zap.Int64("target-asg-desired-capacity", update.ASGDesiredCapacity),
		zap.Int64("target-asg-max-size", update.ASGMaxSize),
		zap.String("update-id", update.ID),
		zap.String("request-id", reqID),
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
		cur.Status = fmt.Sprintf("scale failed %v", err)
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[mngName] = cur
		ts.cfg.EKSConfig.Sync()
		return fmt.Errorf("MNGs[%q] scale failed %v", mngName, err)
	}

	ts.cfg.Logger.Info("successfully scale updated MNG", zap.String("update-id", update.ID), zap.String("mng-name", mngName))
	ts.cfg.EKSConfig.Sync()
	return nil
}
