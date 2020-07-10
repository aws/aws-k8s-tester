// Package cfn implements common CloudFormation utilities.
package cfn

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
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// StackStatus represents the CloudFormation status.
type StackStatus struct {
	Stack *cloudformation.Stack
	Error error
}

// Poll periodically fetches the stack status
// until the stack becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	logWriter io.Writer,
	cfnAPI cloudformationiface.CloudFormationAPI,
	stackID string,
	desiredStackStatus string,
	initialWait time.Duration,
	pollInterval time.Duration,
) <-chan StackStatus {
	now := time.Now()
	sp := spinner.New("Waiting for CFN stack "+desiredStackStatus, logWriter)

	lg.Info("polling stack",
		zap.String("stack-id", stackID),
		zap.String("want", desiredStackStatus),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
	)
	ch := make(chan StackStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		interval := time.Duration(0)

		prevStatusReason, first := "", true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
				ch <- StackStatus{Stack: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
				ch <- StackStatus{Stack: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(interval):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if interval == time.Duration(0) {
					interval = pollInterval
				}
			}

			output, err := cfnAPI.DescribeStacks(&cloudformation.DescribeStacksInput{
				StackName: aws.String(stackID),
			})
			if err != nil {
				if StackNotExist(err) {
					if desiredStackStatus == cloudformation.ResourceStatusDeleteComplete {
						lg.Info("stack is already deleted as desired; exiting", zap.Error(err))
						ch <- StackStatus{Stack: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("stack does not exist; aborting", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: err}
					close(ch)
					return
				}

				lg.Warn("describe stack failed; retrying", zap.Error(err))
				ch <- StackStatus{Stack: nil, Error: err}
				continue
			}

			if len(output.Stacks) != 1 {
				lg.Warn("expected only 1 stack; retrying", zap.String("stacks", output.GoString()))
				ch <- StackStatus{Stack: nil, Error: fmt.Errorf("unexpected stack response %+v", output.GoString())}
				continue
			}

			stack := output.Stacks[0]
			currentStatus := aws.StringValue(stack.StackStatus)
			currentStatusReason := aws.StringValue(stack.StackStatusReason)
			if prevStatusReason == "" {
				prevStatusReason = currentStatusReason
			} else if currentStatusReason != "" && prevStatusReason != currentStatusReason {
				prevStatusReason = currentStatusReason
			}

			lg.Info("poll",
				zap.String("name", aws.StringValue(stack.StackName)),
				zap.String("desired", desiredStackStatus),
				zap.String("current", currentStatus),
				zap.String("current-reason", currentStatusReason),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
			)

			if desiredStackStatus != cloudformation.ResourceStatusDeleteComplete &&
				currentStatus == cloudformation.ResourceStatusDeleteComplete {
				lg.Warn("create stack failed; aborting")
				ch <- StackStatus{
					Stack: stack,
					Error: fmt.Errorf("stack failed thus deleted (previous status reason %q, current stack status %q, current status reason %q)",
						prevStatusReason,
						currentStatus,
						currentStatusReason,
					)}
				close(ch)
				return
			}

			if desiredStackStatus == cloudformation.ResourceStatusDeleteComplete &&
				currentStatus == cloudformation.ResourceStatusDeleteFailed {
				lg.Warn("delete stack failed; aborting")
				ch <- StackStatus{
					Stack: stack,
					Error: fmt.Errorf("failed to delete stack (previous status reason %q, current stack status %q, current status reason %q)",
						prevStatusReason,
						currentStatus,
						currentStatusReason,
					)}
				close(ch)
				return
			}

			ch <- StackStatus{Stack: stack, Error: nil}
			if currentStatus == desiredStackStatus {
				lg.Info("desired stack status; done", zap.String("current-stack-status", currentStatus))
				close(ch)
				return
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				sp.Restart()
				select {
				case <-ctx.Done():
					sp.Stop()
					lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: ctx.Err()}
					close(ch)
					return
				case <-stopc:
					sp.Stop()
					lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: errors.New("wait stopped")}
					close(ch)
					return
				case <-time.After(initialWait):
					sp.Stop()
				}
				first = false
			}

			// continue for-loop
		}
		lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
		ch <- StackStatus{Stack: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// StackCreateFailed return true if cloudformation status indicates its creation failure.
//
//   CREATE_IN_PROGRESS
//   CREATE_FAILED
//   CREATE_COMPLETE
//   ROLLBACK_IN_PROGRESS
//   ROLLBACK_FAILED
//   ROLLBACK_COMPLETE
//   DELETE_IN_PROGRESS
//   DELETE_FAILED
//   DELETE_COMPLETE
//   UPDATE_IN_PROGRESS
//   UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
//   UPDATE_COMPLETE
//   UPDATE_ROLLBACK_IN_PROGRESS
//   UPDATE_ROLLBACK_FAILED
//   UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
//   UPDATE_ROLLBACK_COMPLETE
//   REVIEW_IN_PROGRESS
//
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html
//
func StackCreateFailed(status string) bool {
	return !strings.HasPrefix(status, "REVIEW_") && !strings.HasPrefix(status, "CREATE_")
}

// StackNotExist returns true if cloudformation errror indicates
// that the stack has already been deleted.
// This message is Go client specific.
// e.g. ValidationError: Stack with id AWSTESTER-155460CAAC98A17003-CF-STACK-VPC does not exist\n\tstatus code: 400, request id: bf45410b-b863-11e8-9550-914acc220b7c
func StackNotExist(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ValidationError:") && strings.Contains(err.Error(), " does not exist")
}

// NewTags returns a list of default CloudFormation tags.
func NewTags(input map[string]string) (tags []*cloudformation.Tag) {
	for k, v := range input {
		tags = append(tags, &cloudformation.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags
}
