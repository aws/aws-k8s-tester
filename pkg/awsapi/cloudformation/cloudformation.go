// Package cloudformation implements common CloudFormation utilities.
package cloudformation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	svccfn "github.com/aws/aws-sdk-go/service/cloudformation"
	gocfn "github.com/awslabs/goformation/v3/cloudformation"
	"github.com/awslabs/goformation/v3/cloudformation/tags"
	"github.com/awslabs/goformation/v3/intrinsics"
	"github.com/sanathkr/yaml"
	"go.uber.org/zap"
)

// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/pseudo-parameter-reference.html
const (
	PseudoParameterAccountId        = "AWS::AccountId"
	PseudoParameterNotificationARNs = "AWS::NotificationARNs"
	PseudoParameterNoValue          = "AWS::NoValue"
	PseudoParameterPartition        = "AWS::Partition"
	PseudoParameterRegion           = "AWS::Region"
	PseudoParameterStackId          = "AWS::StackId"
	PseudoParameterStackName        = "AWS::StackName"
	PseudoParameterURLSuffix        = "AWS::URLSuffix"
)

// Parameter is the CloudFormation template output.
type Parameter struct {
	Type        string `json:"Type,omitempty"`
	Default     string `json:"Default,omitempty"`
	Description string `json:"Description,omitempty"`
}

// Output is the CloudFormation template output.
type Output struct {
	Description string            `json:"Description,omitempty"`
	Value       string            `json:"Value,omitempty"`
	Export      map[string]string `json:"Export,omitempty"`
}

// Interface represents "AWS::CloudFormation::Interface".
type Interface struct {
	ParameterGroups []ParameterGroupEntry `json:"ParameterGroups,omitempty"`
}

// ParameterGroupEntry is the CloudFormation template entry.
type ParameterGroupEntry struct {
	Label      map[string]string `json:"Label,omitempty"`
	Parameters []string          `json:"Parameters,omitempty"`
}

// NewCloudFormationTags returns a list of default CloudFormation tags.
func NewCloudFormationTags(input map[string]string) (tags []*svccfn.Tag) {
	for k, v := range input {
		tags = append(tags, &svccfn.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags
}

// NewTags returns a list of default CloudFormation tags.
func NewTags(input map[string]string) (rs []tags.Tag) {
	for k, v := range input {
		rs = append(rs, tags.Tag{Key: k, Value: v})
	}
	return rs
}

const (
	// ConditionKeyHas2Azs is the Condition key for "Has2Azs".
	ConditionKeyHas2Azs = "Has2Azs"
	// ConditionKeyHasMoreThan2Azs is the Condition key for "HasMoreThan2Azs".
	ConditionKeyHasMoreThan2Azs = "HasMoreThan2Azs"
)

// NewConditionsForAZ returns the new conditions for AZ.
func NewConditionsForAZ() map[string]interface{} {
	return map[string]interface{}{
		ConditionKeyHas2Azs: gocfn.Or([]string{
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"ap-south-1",
			),
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"ap-northeast-2",
			),
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"ca-central-1",
			),
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"cn-north-1",
			),
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"sa-east-1",
			),
			gocfn.Equals(
				gocfn.Ref(PseudoParameterRegion),
				"us-west-1",
			),
		}),
		ConditionKeyHasMoreThan2Azs: gocfn.Not([]string{ConditionKeyHas2Azs}),
	}
}

// NewConditionsForAZText returns the new conditions for AZ.
func NewConditionsForAZText() map[string]interface{} {
	return map[string]interface{}{
		ConditionKeyHas2Azs: []FnOrEntry{
			{FnEquals: []string{
				`!Ref AWS::Region`,
				"ap-south-1",
			}},
		},
		ConditionKeyHasMoreThan2Azs: []FnNotEntry{
			{Condition: ConditionKeyHas2Azs},
		},
	}
}

// FnOrEntry implements "Or" intrinsic function entry.
type FnOrEntry struct {
	FnEquals []string `json:"FnEquals,omitempty"`
}

// FnNotEntry implements "Not" intrinsic function entry.
type FnNotEntry struct {
	Condition string `json:"Condition,omitempty"`
}

// StackStatus represents the CloudFormation status.
type StackStatus struct {
	Stack *svccfn.Stack
	Error error
}

// Wait waits until the stack becomes the desired state.
func Wait(
	ctx context.Context,
	lg *zap.Logger,
	svc *svccfn.CloudFormation,
	stackID string,
	desiredStackStatus string,
) <-chan StackStatus {
	lg.Info("start waiting for stack",
		zap.String("stack-id", stackID),
		zap.String("desired-stack-status", desiredStackStatus),
	)

	ch := make(chan StackStatus, 10)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		prevStatusReason := ""
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- StackStatus{Stack: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-ticker.C:
			}

			output, err := svc.DescribeStacks(&svccfn.DescribeStacksInput{
				StackName: aws.String(stackID),
			})
			if err != nil {
				if IsStackDeleted(err) {
					lg.Warn("stack does not exist", zap.Error(ctx.Err()))
					ch <- StackStatus{Stack: nil, Error: err}
					close(ch)
					return
				}

				lg.Error("describe stack failed", zap.Error(err))
				ch <- StackStatus{Stack: nil, Error: err}
				continue
			}

			if len(output.Stacks) != 1 {
				lg.Error("expected only 1 stack", zap.String("stacks", fmt.Sprintf("%+v", output.GoString())))
				ch <- StackStatus{Stack: nil, Error: fmt.Errorf("unexpected stack response %+v", output.GoString())}
				continue
			}

			stack := output.Stacks[0]
			status := aws.StringValue(stack.StackStatus)
			currentStatusReason := aws.StringValue(stack.StackStatusReason)
			if prevStatusReason == "" {
				prevStatusReason = currentStatusReason
			} else if prevStatusReason != "" && currentStatusReason != "" && prevStatusReason != currentStatusReason {
				prevStatusReason = currentStatusReason
			}

			lg.Info("described",
				zap.String("stack-name", aws.StringValue(stack.StackName)),
				zap.String("stack-id", aws.StringValue(stack.StackId)),
				zap.String("stack-status", status),
			)
			if status == svccfn.ResourceStatusDeleteComplete && desiredStackStatus != svccfn.ResourceStatusDeleteComplete {
				lg.Error("create stack failed")
				ch <- StackStatus{Stack: stack, Error: fmt.Errorf("stack failed thus deleted (%v)", prevStatusReason)}
				close(ch)
				return
			}

			ch <- StackStatus{Stack: stack, Error: nil}
			if status == desiredStackStatus {
				lg.Info("became desired stack status", zap.String("stack-status", status))
				close(ch)
				return
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- StackStatus{Stack: nil, Error: ctx.Err()}
		close(ch)
		return
	}()

	return ch
}

// IsStackCreateFailed return true if cloudformation status indicates its creation failure.
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html
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
func IsStackCreateFailed(status string) bool {
	return !strings.HasPrefix(status, "REVIEW_") && !strings.HasPrefix(status, "CREATE_")
}

// IsStackDeleted returns true if cloudformation errror indicates
// that the stack has already been deleted.
// This message is Go client specific.
// e.g. ValidationError: Stack with id AWSTESTER-155460CAAC98A17003-CF-STACK-VPC does not exist\n\tstatus code: 400, request id: bf45410b-b863-11e8-9550-914acc220b7c
func IsStackDeleted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ValidationError:") && strings.Contains(err.Error(), " does not exist")
}

// ConvertToYAMLWithConditions converts template to YAML.
// ref. https://github.com/awslabs/goformation/issues/157
func ConvertToYAMLWithConditions(tmpl *gocfn.Template) ([]byte, error) {
	body, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return nil, err
	}
	body, err = intrinsics.ProcessJSON(body, &intrinsics.ProcessorOptions{
		IntrinsicHandlerOverrides: gocfn.EncoderIntrinsics,
		NoProcess:                 true,
	})
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(body)
}
