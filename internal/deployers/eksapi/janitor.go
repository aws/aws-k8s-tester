package eksapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-k8s-tester/internal/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"k8s.io/klog/v2"
)

func NewJanitor(maxResourceAge time.Duration, emitMetrics bool, workers int, stackStatus string) *janitor {
	awsConfig := awssdk.NewConfig()
	var metricRegistry metrics.MetricRegistry
	if emitMetrics {
		metricRegistry = metrics.NewCloudWatchRegistry(cloudwatch.NewFromConfig(awsConfig))
	} else {
		metricRegistry = metrics.NewNoopMetricRegistry()
	}
	if workers <= 0 {
		workers = 1
	}
	return &janitor{
		maxResourceAge: maxResourceAge,
		workers:        workers,
		stackStatus:    stackStatus,
		awsConfig:      awsConfig,
		cfnClient:      cloudformation.NewFromConfig(awsConfig),
		metrics:        metricRegistry,
	}
}

type janitor struct {
	maxResourceAge time.Duration
	workers        int
	stackStatus    string

	awsConfig aws.Config
	cfnClient *cloudformation.Client
	metrics   metrics.MetricRegistry
}

func (j *janitor) Sweep(ctx context.Context) error {
	awsConfig := awssdk.NewConfig()
	cfnClient := cloudformation.NewFromConfig(awsConfig)
	stacks, err := j.getStacks(ctx, cfnClient)
	if err != nil {
		return fmt.Errorf("failed to get stacks: %v", err)
	}
	var wg sync.WaitGroup
	stackQueue := make(chan cloudformationtypes.Stack, len(stacks))
	errChan := make(chan error, len(stacks))
	for i := 1; i <= j.workers; i++ {
		wg.Add(1)
		go j.sweepWorker(&wg, stackQueue, errChan)
	}

	for _, stack := range stacks {
		stackQueue <- stack
	}
	close(stackQueue)

	wg.Wait()
	close(errChan)
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (j *janitor) getStacks(ctx context.Context, cfnClient *cloudformation.Client) ([]cloudformationtypes.Stack, error) {
	var stacks []cloudformationtypes.Stack
	stackPaginator := cloudformation.NewDescribeStacksPaginator(cfnClient, &cloudformation.DescribeStacksInput{})
	for stackPaginator.HasMorePages() {
		page, err := stackPaginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		stacks = append(stacks, page.Stacks...)
	}
	return stacks, nil
}

func (j *janitor) sweepWorker(wg *sync.WaitGroup, stackQueue <-chan cloudformationtypes.Stack, errChan chan<- error) {
	defer wg.Done()
	for stack := range stackQueue {
		resourceID := *stack.StackName
		if !strings.HasPrefix(resourceID, ResourcePrefix) {
			continue
		}
		if stack.StackStatus == "DELETE_COMPLETE" {
			continue
		}
		if j.stackStatus != "" && j.stackStatus != string(stack.StackStatus) {
			klog.Infof("skipping resources (status: %v): %s", stack.StackStatus, resourceID)
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
		if err := deleteResources(infraManager, clusterManager, nodeManager, nil /* k8sClient */, nil /* deployerOptions */); err != nil {
			errChan <- fmt.Errorf("failed to delete resources: %s: %v", resourceID, err)
		}
	}
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
