// Package wait implements cluster waiter.
package wait

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// IsDeleted returns true if error from EKS API indicates that
// the EKS cluster has already been deleted.
func IsDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" &&
		strings.HasPrefix(awsErr.Message(), "No cluster found for") {
		return true
	}
	// ResourceNotFoundException: No cluster found for name: aws-k8s-tester-155468BC717E03B003\n\tstatus code: 404, request id: 1e3fe41c-b878-11e8-adca-b503e0ba731d
	return strings.Contains(err.Error(), "No cluster found for name: ")
}

// ClusterStatus represents the EKS cluster status.
type ClusterStatus struct {
	Cluster *aws_eks.Cluster
	Error   error
}

// Poll periodically fetches the cluster status
// until the cluster becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	desiredClusterStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan ClusterStatus {
	lg.Info("polling cluster",
		zap.String("cluster-name", clusterName),
		zap.String("desired-status", desiredClusterStatus),
	)

	now := time.Now()

	ch := make(chan ClusterStatus, 10)
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
				ch <- ClusterStatus{Cluster: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ClusterStatus{Cluster: nil, Error: errors.New("wait stopped")}
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

			output, err := eksAPI.DescribeCluster(&aws_eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil {
				if IsDeleted(err) {
					if desiredClusterStatus == eksconfig.ClusterStatusDELETEDORNOTEXIST {
						lg.Info("cluster is already deleted as desired; exiting", zap.Error(err))
						ch <- ClusterStatus{Cluster: nil, Error: nil}
						close(ch)
						return
					}
					lg.Warn("cluster does not exist; aborting", zap.Error(err))
					ch <- ClusterStatus{Cluster: nil, Error: err}
					close(ch)
					return
				}
				lg.Warn("describe cluster failed; retrying", zap.Error(err))
				ch <- ClusterStatus{Cluster: nil, Error: err}
				continue
			}

			if output.Cluster == nil {
				lg.Warn("expected non-nil cluster; retrying")
				ch <- ClusterStatus{Cluster: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			cluster := output.Cluster
			currentStatus := aws.StringValue(cluster.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredClusterStatus:
				ch <- ClusterStatus{Cluster: cluster, Error: nil}
				lg.Info("desired cluster status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case aws_eks.ClusterStatusFailed:
				ch <- ClusterStatus{Cluster: cluster, Error: fmt.Errorf("unexpected cluster status %q", aws_eks.ClusterStatusFailed)}
				lg.Warn("cluster status failed", zap.String("status", currentStatus), zap.String("desired-status", desiredClusterStatus))
				close(ch)
				return
			default:
				ch <- ClusterStatus{Cluster: cluster, Error: nil}
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- ClusterStatus{Cluster: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- ClusterStatus{Cluster: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ClusterStatus{Cluster: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}
