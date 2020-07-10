package wait

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/ctxutil"
	"github.com/aws/aws-k8s-tester/pkg/spinner"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// ManagedNodeGroupStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
//
//  CREATING
//  ACTIVE
//  DELETING
//  FAILED
//  UPDATING
//
const ManagedNodeGroupStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// ManagedNodeGroupStatus represents the CloudFormation status.
type ManagedNodeGroupStatus struct {
	NodeGroupName string
	NodeGroup     *aws_eks.Nodegroup
	Error         error
}

// Poll periodically fetches the managed node group status
// until the node group becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	logWriter io.Writer,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	mngName string,
	desiredNodeGroupStatus string,
	initialWait time.Duration,
	pollInterval time.Duration,
	opts ...OpOption) <-chan ManagedNodeGroupStatus {

	ret := Op{}
	ret.applyOpts(opts)

	now := time.Now()
	sp := spinner.New("Waiting for Managed Node Group status "+desiredNodeGroupStatus, logWriter)

	lg.Info("polling mng",
		zap.String("cluster-name", clusterName),
		zap.String("mng-name", mngName),
		zap.String("desired-status", desiredNodeGroupStatus),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
	)

	ch := make(chan ManagedNodeGroupStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		waitDur := time.Duration(0)

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = pollInterval
				}
			}

			output, err := eksAPI.DescribeNodegroup(&aws_eks.DescribeNodegroupInput{
				ClusterName:   aws.String(clusterName),
				NodegroupName: aws.String(mngName),
			})
			if err != nil {
				if IsDeleted(err) {
					if desiredNodeGroupStatus == ManagedNodeGroupStatusDELETEDORNOTEXIST {
						lg.Info("managed node group is already deleted as desired; exiting", zap.Error(err))
						ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: nil}
						close(ch)
						return
					}
					lg.Warn("managed node group does not exist", zap.Error(err))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
					close(ch)
					return
				}
				lg.Warn("describe managed node group failed; retrying", zap.Error(err))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: err}
				continue
			}

			if output.Nodegroup == nil {
				lg.Warn("expected non-nil managed node group; retrying")
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			nodeGroup := output.Nodegroup
			currentStatus := aws.StringValue(nodeGroup.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("mng-name", mngName),
				zap.String("status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
			)
			switch currentStatus {
			case desiredNodeGroupStatus:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
				lg.Info("desired managed node group status; done", zap.String("status", currentStatus))
				close(ch)
				return

			case aws_eks.NodegroupStatusCreateFailed,
				aws_eks.NodegroupStatusDeleteFailed,
				aws_eks.NodegroupStatusDegraded:
				lg.Warn("unexpected managed node group status; failed", zap.String("status", currentStatus))
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: fmt.Errorf("unexpected mng status %q", currentStatus)}
				close(ch)
				return

			default:
				ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nodeGroup, Error: nil}
			}

			if ret.queryFunc != nil {
				ret.queryFunc()
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				sp.Restart()
				select {
				case <-ctx.Done():
					sp.Stop()
					lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					sp.Stop()
					lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
					ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
					sp.Stop()
				}
				first = false
			}
		}

		lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
		ch <- ManagedNodeGroupStatus{NodeGroupName: mngName, NodeGroup: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// IsDeleted returns true if error from EKS API indicates that
// the EKS managed node group has already been deleted.
func IsDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" {
		return true
	}

	// ResourceNotFoundException: nodeGroup eks-2019120505-pdx-us-west-2-tqy2d-managed-node-group not found for cluster eks-2019120505-pdx-us-west-2-tqy2d\n\tstatus code: 404, request id: 330998c1-22e9-4a8b-b180-420dadade090
	return strings.Contains(err.Error(), "No cluster found for") ||
		strings.Contains(err.Error(), " not found for cluster ")
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

// PollUpdate periodically fetches the MNG update status
// until the MNG update becomes the desired state.
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeUpdate.html
func PollUpdate(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	logWriter io.Writer,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	mngName string,
	requestID string,
	desiredUpdateStatus string,
	initialWait time.Duration,
	pollInterval time.Duration,
	opts ...OpOption) <-chan UpdateStatus {

	ret := Op{}
	ret.applyOpts(opts)

	now := time.Now()
	sp := spinner.New("Waiting for Managed Node Group update status "+desiredUpdateStatus, logWriter)

	lg.Info("polling mng update",
		zap.String("cluster-name", clusterName),
		zap.String("mng-name", mngName),
		zap.String("request-id", requestID),
		zap.String("desired-update-status", desiredUpdateStatus),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
	)

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
				lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
				ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
				ch <- UpdateStatus{Update: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = pollInterval
				}
			}

			output, err := eksAPI.DescribeUpdate(&eks.DescribeUpdateInput{
				Name:          aws.String(clusterName),
				NodegroupName: aws.String(mngName),
				UpdateId:      aws.String(requestID),
			})
			if err != nil {
				if updateNotExists(err) {
					lg.Warn("mng update does not exist; aborting", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe mng update failed; retrying", zap.Error(err))
				ch <- UpdateStatus{Update: nil, Error: err}
				continue
			}

			if output.Update == nil {
				lg.Warn("expected non-nil mng update; retrying")
				ch <- UpdateStatus{Update: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			update := output.Update
			currentStatus := aws.StringValue(update.Status)
			updateType := aws.StringValue(update.Type)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("mng-name", mngName),
				zap.String("status", currentStatus),
				zap.String("update-type", updateType),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
			)
			switch currentStatus {
			case desiredUpdateStatus:
				ch <- UpdateStatus{Update: update, Error: nil}
				lg.Info("desired mng update status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case eks.UpdateStatusCancelled:
				ch <- UpdateStatus{Update: update, Error: fmt.Errorf("unexpected mng update status %q", eks.UpdateStatusCancelled)}
				lg.Warn("mng update status cancelled", zap.String("status", currentStatus), zap.String("desired-status", desiredUpdateStatus))
				close(ch)
				return
			case eks.UpdateStatusFailed:
				ch <- UpdateStatus{Update: update, Error: fmt.Errorf("unexpected mng update status %q", eks.UpdateStatusFailed)}
				lg.Warn("mng update status failed", zap.String("status", currentStatus), zap.String("desired-status", desiredUpdateStatus))
				close(ch)
				return
			default:
				ch <- UpdateStatus{Update: update, Error: nil}
			}

			if ret.queryFunc != nil {
				ret.queryFunc()
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				sp.Restart()
				select {
				case <-ctx.Done():
					sp.Stop()
					lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					sp.Stop()
					lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
					ch <- UpdateStatus{Update: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
					sp.Stop()
				}
				first = false
			}
		}

		lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
		ch <- UpdateStatus{Update: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// Op represents a MNG operation.
type Op struct {
	queryFunc func()
}

// OpOption configures archiver operations.
type OpOption func(*Op)

// WithQueryFunc configures query function to be called in retry func.
func WithQueryFunc(f func()) OpOption {
	return func(op *Op) { op.queryFunc = f }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
