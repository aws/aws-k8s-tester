package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	types "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// TODO: implement AWS client wrappers, and incorporate this into the cfn:CreateStack call
func WrapCFNStackFailure(ctx context.Context, cfnClient *cloudformation.Client, createStackErr error, stackName string) error {
	if createStackErr == nil {
		return nil
	}
	resourceByFailureMode := make(map[string][]string)
	eventsPaginator := cloudformation.NewDescribeStackEventsPaginator(cfnClient, &cloudformation.DescribeStackEventsInput{
		StackName: &stackName,
	})
	for eventsPaginator.HasMorePages() {
		page, err := eventsPaginator.NextPage(ctx)
		if err != nil {
			return createStackErr
		}
		for _, event := range page.StackEvents {
			if event.ResourceStatus == types.ResourceStatusCreateFailed {
				if _, ok := resourceByFailureMode[aws.ToString(event.ResourceStatusReason)]; !ok {
					resourceByFailureMode[aws.ToString(event.ResourceStatusReason)] = []string{}
				}
				resourceByFailureMode[aws.ToString(event.ResourceStatusReason)] = append(resourceByFailureMode[aws.ToString(event.ResourceStatusReason)], aws.ToString(event.LogicalResourceId))
			}
		}
	}
	nonCancellationFailure := len(resourceByFailureMode) > 1
	var enhancedDetails []string
	for reason, resources := range resourceByFailureMode {
		if nonCancellationFailure && reason == "Resource creation cancelled" {
			// Ignore resource cancellation errors if there's another failure reported, those failures
			// would just be a consequence of that failure. If all the failures are resource cancellation,
			// then there was likely a user initiated delete of the whole stack based on a timeout
			// waiting for one of the resources to create
			continue
		}
		enhancedDetails = append(enhancedDetails, fmt.Sprintf("%s: %s", strings.Join(resources, ","), reason))
	}
	return fmt.Errorf("%w: %s", createStackErr, strings.Join(enhancedDetails, "--"))
}
