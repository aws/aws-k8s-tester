package eksapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-k8s-tester/internal/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func NewJanitor(maxResourceAge time.Duration, emitMetrics bool, workers int, stackStatus string, allRegions bool) *janitor {
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
		allRegions:     allRegions,
		awsConfig:      awsConfig,
		metrics:        metricRegistry,
	}
}

type janitor struct {
	maxResourceAge time.Duration
	workers        int
	stackStatus    string
	// allRegions, when true, sweeps every region enabled for the account.
	// When false, only the default region from the AWS config is swept.
	allRegions bool

	awsConfig aws.Config
	metrics   metrics.MetricRegistry
}

// Sweep sweeps the selected regions. When allRegions is set, every region
// enabled for the account is discovered and swept; otherwise only the default
// region from the AWS config is swept. The janitor can run in any single region
// and still reach every other region, because the target region is determined
// by the AWS SDK config, not by where the process runs.
func (j *janitor) Sweep(ctx context.Context) error {
	var regions []string
	if j.allRegions {
		discovered, err := j.getRegions(ctx)
		if err != nil {
			return fmt.Errorf("failed to get regions: %v", err)
		}
		regions = discovered
	} else {
		regions = []string{j.awsConfig.Region}
	}
	slog.Info("sweeping regions", "regions", regions)
	var errs []error
	for _, region := range regions {
		if err := j.sweepRegion(ctx, region); err != nil {
			errs = append(errs, fmt.Errorf("region %s: %v", region, err))
		}
	}
	return errors.Join(errs...)
}

// getRegions returns the regions that are enabled (or opted in) for the
// account. Disabled opt-in regions are excluded.
func (j *janitor) getRegions(ctx context.Context) ([]string, error) {
	ec2Client := ec2.NewFromConfig(j.awsConfig)
	out, err := ec2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}
	var regions []string
	for _, r := range out.Regions {
		regions = append(regions, *r.RegionName)
	}
	return regions, nil
}

// sweepRegion sweeps a single region using a region-scoped copy of the base
// AWS config.
func (j *janitor) sweepRegion(ctx context.Context, region string) error {
	cfg := j.awsConfig.Copy()
	cfg.Region = region
	cfnClient := cloudformation.NewFromConfig(cfg)
	stacks, err := j.getStacks(ctx, cfnClient)
	if err != nil {
		return fmt.Errorf("failed to get stacks: %v", err)
	}
	var wg sync.WaitGroup
	stackQueue := make(chan cloudformationtypes.Stack, len(stacks))
	errChan := make(chan error, len(stacks))
	for i := 1; i <= j.workers; i++ {
		wg.Add(1)
		go j.sweepWorker(cfg, &wg, stackQueue, errChan)
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

func (j *janitor) sweepWorker(cfg aws.Config, wg *sync.WaitGroup, stackQueue <-chan cloudformationtypes.Stack, errChan chan<- error) {
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
			slog.Info("skipping resources", "status", stack.StackStatus, "resourceID", resourceID)
			continue
		}
		resourceAge := time.Since(*stack.CreationTime)
		if resourceAge < j.maxResourceAge {
			slog.Info("skipping resources", "age", resourceAge, "resourceID", resourceID)
			continue
		}
		clients := j.awsClientsForStack(cfg, stack)
		infraManager := NewInfrastructureManager(clients, resourceID, j.metrics)
		clusterManager := NewClusterManager(clients, resourceID)
		nodeManager := NewNodeManager(clients, resourceID)
		slog.Info("deleting resources", "age", resourceAge, "resourceID", resourceID, "region", cfg.Region)
		if err := deleteResources(infraManager, clusterManager, nodeManager, nil /* k8sClient */, nil /* deployerOptions */); err != nil {
			errChan <- fmt.Errorf("failed to delete resources: %s: %v", resourceID, err)
		}
	}
}

func (j *janitor) awsClientsForStack(cfg aws.Config, stack cloudformationtypes.Stack) *awsClients {
	var eksEndpointURL string
	for _, tag := range stack.Tags {
		if *tag.Key == eksEndpointURLTag {
			eksEndpointURL = *tag.Value
		}
	}
	return newAWSClients(cfg, eksEndpointURL)
}
