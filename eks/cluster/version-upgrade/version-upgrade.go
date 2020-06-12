// Package versionupgrade implements EKS cluster version upgrade tester.
package versionupgrade

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
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
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	var updateOut *eks.UpdateClusterVersionOutput
	updateOut, err = ts.cfg.EKSAPI.UpdateClusterVersion(&eks.UpdateClusterVersionInput{
		Name:    aws.String(ts.cfg.EKSConfig.Name),
		Version: aws.String(ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	})
	if err != nil {
		return err
	}
	reqID := ""
	if updateOut.Update != nil {
		reqID = aws.StringValue(updateOut.Update.Id)
	}
	ts.cfg.Logger.Info("sent upgrade cluster request", zap.String("request-id", reqID))

	// takes ~30-min
	initialWait := 10 * time.Minute

	// enough time for upgrade fail/rollback
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour+30*time.Minute)
	ch := Poll(
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
	for v := range ch {
		err = v.Error
	}
	cancel()
	if err != nil {
		return err
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
	if err == nil {
		ts.cfg.Logger.Info("health check success after cluster version upgrade")
	} else {
		ts.cfg.Logger.Warn("health check failed after cluster version upgrade", zap.Error(err))
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

// updateNotExists returns true if error from EKS API indicates that
// the EKS cluster update does not exist.
func updateNotExists(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" &&
		strings.HasPrefix(awsErr.Message(), "No update found for") {
		return true
	}
	// An error occurred (ResourceNotFoundException) when calling the DescribeUpdate operation: No update found for ID: 10bddb13-a71b-425a-b0a6-71cd03e59161
	return strings.Contains(err.Error(), "No update found")
}

// UpdateStatus represents the CloudFormation status.
type UpdateStatus struct {
	Update *eks.Update
	Error  error
}

// Poll periodically fetches the cluster update status
// until the cluster update becomes the desired state.
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeUpdate.html
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	requestID string,
	desiredUpdateStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan UpdateStatus {
	lg.Info("polling cluster update",
		zap.String("cluster-name", clusterName),
		zap.String("request-id", requestID),
		zap.String("desired-update-status", desiredUpdateStatus),
	)

	now := time.Now()

	ch := make(chan UpdateStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		waitDur := time.Duration(0)

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- UpdateStatus{Update: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = wait
				}
			}

			output, err := eksAPI.DescribeUpdate(&eks.DescribeUpdateInput{
				Name:     aws.String(clusterName),
				UpdateId: aws.String(requestID),
			})
			if err != nil {
				if updateNotExists(err) {
					lg.Warn("cluster update does not exist; aborting", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe cluster failed; retrying", zap.Error(err))
				ch <- UpdateStatus{Update: nil, Error: err}
				continue
			}

			if output.Update == nil {
				lg.Warn("expected non-nil cluster; retrying")
				ch <- UpdateStatus{Update: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			update := output.Update
			currentStatus := aws.StringValue(update.Status)
			updateType := aws.StringValue(update.Type)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("status", currentStatus),
				zap.String("update-type", updateType),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredUpdateStatus:
				ch <- UpdateStatus{Update: update, Error: nil}
				lg.Info("desired cluster update status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case eks.UpdateStatusCancelled:
				ch <- UpdateStatus{Update: update, Error: fmt.Errorf("unexpected cluster update status %q", eks.UpdateStatusCancelled)}
				lg.Warn("cluster update status cancelled", zap.String("status", currentStatus), zap.String("desired-status", desiredUpdateStatus))
				close(ch)
				return
			case eks.UpdateStatusFailed:
				ch <- UpdateStatus{Update: update, Error: fmt.Errorf("unexpected cluster update status %q", eks.UpdateStatusFailed)}
				lg.Warn("cluster update status failed", zap.String("status", currentStatus), zap.String("desired-status", desiredUpdateStatus))
				close(ch)
				return
			default:
				ch <- UpdateStatus{Update: update, Error: nil}
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}
