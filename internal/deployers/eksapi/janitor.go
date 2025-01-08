package eksapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-k8s-tester/internal/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"k8s.io/klog/v2"
)

func NewJanitor(maxResourceAge time.Duration, emitMetrics bool) *janitor {
	awsConfig := awssdk.NewConfig()
	var metricRegistry metrics.MetricRegistry
	if emitMetrics {
		metricRegistry = metrics.NewCloudWatchRegistry(cloudwatch.NewFromConfig(awsConfig))
	} else {
		metricRegistry = metrics.NewNoopMetricRegistry()
	}
	return &janitor{
		maxResourceAge: maxResourceAge,
		awsConfig:      awsConfig,
		cfnClient:      cloudformation.NewFromConfig(awsConfig),
		metrics:        metricRegistry,
	}
}

type janitor struct {
	maxResourceAge time.Duration

	awsConfig aws.Config
	cfnClient *cloudformation.Client
	metrics   metrics.MetricRegistry
}

func (j *janitor) Sweep(ctx context.Context) error {
	awsConfig := awssdk.NewConfig()
	cfnClient := cloudformation.NewFromConfig(awsConfig)
	stacks := cloudformation.NewDescribeStacksPaginator(cfnClient, &cloudformation.DescribeStacksInput{})
	var errs []error
	for stacks.HasMorePages() {
		page, err := stacks.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, stack := range page.Stacks {
			resourceID := *stack.StackName
			if !strings.HasPrefix(resourceID, ResourcePrefix) {
				continue
			}
			if stack.StackStatus == "DELETE_COMPLETE" {
				continue
			}
			resourceAge := time.Since(*stack.CreationTime)
			if resourceAge < j.maxResourceAge {
				klog.Infof("skipping resources (%v old): %s", resourceAge, resourceID)
				continue
			}
			clients := j.awsClientsForStack(stack)
			infraManager := NewInfrastructureManager(clients, resourceID, j.metrics)
			clusterManager := NewClusterManager(clients, resourceID)
			nodeManager := NewNodeManager(clients, resourceID)
			klog.Infof("deleting resources (%v old): %s", resourceAge, resourceID)
			if err := deleteResources(infraManager, clusterManager, nodeManager /* TODO: pass a k8sClient */, nil, nil); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete resources: %s: %v", resourceID, err))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (j *janitor) awsClientsForStack(stack cloudformationtypes.Stack) *awsClients {
	var eksEndpointURL string
	for _, tag := range stack.Tags {
		if *tag.Key == eksEndpointURLTag {
			eksEndpointURL = *tag.Value
		}
	}
	return newAWSClients(j.awsConfig, eksEndpointURL)
}
