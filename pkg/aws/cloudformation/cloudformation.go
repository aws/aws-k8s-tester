// Package cloudformation implements common CloudFormation utilities.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	svccfn "github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// StackStatus represents the CloudFormation status.
type StackStatus struct {
	Stack *svccfn.Stack
	Error error
}

// Poll periodically fetches the stack status
// until the stack becomes the desired state.
func Poll(
	ctx context.Context,
	stopc chan struct{},
	osSig chan os.Signal,
	lg *zap.Logger,
	cfnAPI cloudformationiface.CloudFormationAPI,
	stackID string,
	desiredStackStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan StackStatus {
	now := time.Now()

	lg.Info("polling stack",
		zap.String("stack-id", stackID),
		zap.String("want", desiredStackStatus),
	)
	ch := make(chan StackStatus, 10)
	go func() {
		ticker := time.NewTicker(wait)
		defer ticker.Stop()

		prevStatusReason, first := "", true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- StackStatus{Stack: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- StackStatus{Stack: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case sig := <-osSig:
				lg.Warn("wait stopped", zap.String("os-signal", sig.String()))
				ch <- StackStatus{Stack: nil, Error: fmt.Errorf("wait stopped with %s", sig)}
				close(ch)
				return

			case <-ticker.C:
			}

			output, err := cfnAPI.DescribeStacks(&svccfn.DescribeStacksInput{
				StackName: aws.String(stackID),
			})
			if err != nil {
				if StackNotExist(err) {
					if desiredStackStatus == svccfn.ResourceStatusDeleteComplete {
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

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))

				select {
				case <-ctx.Done():
					lg.Warn("wait aborted", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: ctx.Err()}
					close(ch)
					return

				case <-stopc:
					lg.Warn("wait stopped", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: errors.New("wait stopped")}
					close(ch)
					return

				case sig := <-osSig:
					lg.Warn("wait stopped", zap.String("os-signal", sig.String()))
					ch <- StackStatus{Stack: nil, Error: fmt.Errorf("wait stopped with %s", sig)}
					close(ch)
					return

				case <-time.After(initialWait):
				}
				first = false
			}

			lg.Info("polling",
				zap.String("stack-name", aws.StringValue(stack.StackName)),
				zap.String("desired-status", desiredStackStatus),
				zap.String("current-status", currentStatus),
				zap.String("current-status-reason", currentStatusReason),
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)

			if desiredStackStatus != svccfn.ResourceStatusDeleteComplete &&
				currentStatus == svccfn.ResourceStatusDeleteComplete {
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
			if desiredStackStatus == svccfn.ResourceStatusDeleteComplete &&
				currentStatus == svccfn.ResourceStatusDeleteFailed {
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
				lg.Info("became desired stack status; exiting", zap.String("current-stack-status", currentStatus))
				close(ch)
				return
			}
			// continue for-loop
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
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
func NewTags(input map[string]string) (tags []*svccfn.Tag) {
	for k, v := range input {
		tags = append(tags, &svccfn.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags
}
