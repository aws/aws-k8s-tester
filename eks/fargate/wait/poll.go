// Package wait implements Fargate wait polling functions.
package wait

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// FargateProfileStatusDELETEDORNOTEXIST defines the cluster status when the cluster is not found.
//
// ref. https://docs.aws.amazon.com/eks/latest/APIReference/API_FargateProfile.html
//
//  CREATING
//  ACTIVE
//  DELETING
//  CREATE_FAILED
//  DELETE_FAILED
//
const FargateProfileStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"

// FargateProfileStatus represents the CloudFormation status.
type FargateProfileStatus struct {
	FargateProfile *eks.FargateProfile
	Error          error
}

// Poll periodically fetches the fargate profile status
// until the node group becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	profileName string,
	desiredStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan FargateProfileStatus {
	lg.Info("polling fargate profile",
		zap.String("cluster-name", clusterName),
		zap.String("profile-name", profileName),
		zap.String("desired-status", desiredStatus),
	)

	now := time.Now()

	ch := make(chan FargateProfileStatus, 10)
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
				ch <- FargateProfileStatus{FargateProfile: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- FargateProfileStatus{FargateProfile: nil, Error: errors.New("wait stopped")}
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

			output, err := eksAPI.DescribeFargateProfile(&eks.DescribeFargateProfileInput{
				ClusterName:        aws.String(clusterName),
				FargateProfileName: aws.String(profileName),
			})
			if err != nil {
				if IsProfileDeleted(err) {
					if desiredStatus == FargateProfileStatusDELETEDORNOTEXIST {
						lg.Info("fargate profile is already deleted as desired; exiting", zap.Error(err))
						ch <- FargateProfileStatus{FargateProfile: nil, Error: nil}
						close(ch)
						return
					}
					lg.Warn("fargate profile does not exist", zap.Error(err))
					ch <- FargateProfileStatus{FargateProfile: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe fargate profile failed; retrying", zap.Error(err))
				ch <- FargateProfileStatus{FargateProfile: nil, Error: err}
				continue
			}

			if output.FargateProfile == nil {
				lg.Warn("expected non-nil fargate profile; retrying")
				ch <- FargateProfileStatus{FargateProfile: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			fargateProfile := output.FargateProfile
			currentStatus := aws.StringValue(fargateProfile.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("fargate-name", profileName),
				zap.String("status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredStatus:
				ch <- FargateProfileStatus{FargateProfile: fargateProfile, Error: nil}
				lg.Info("desired fargate profile status; done", zap.String("status", currentStatus))
				close(ch)
				return

			case eks.FargateProfileStatusCreateFailed,
				eks.FargateProfileStatusDeleteFailed:
				lg.Warn("unexpected fargate profile status; failed", zap.String("status", currentStatus))
				ch <- FargateProfileStatus{FargateProfile: fargateProfile, Error: fmt.Errorf("unexpected fargate status %q", currentStatus)}
				close(ch)
				return

			default:
				ch <- FargateProfileStatus{FargateProfile: fargateProfile, Error: nil}
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- FargateProfileStatus{FargateProfile: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- FargateProfileStatus{FargateProfile: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- FargateProfileStatus{FargateProfile: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// IsProfileDeleted returns true if error from EKS API indicates that
// the EKS fargate profile has already been deleted.
func IsProfileDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" {
		return true
	}

	return strings.Contains(err.Error(), " not found ")
}
