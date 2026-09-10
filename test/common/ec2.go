//go:build e2e

package common

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// GPUInfo holds the per-node GPU facts we care about for the NVIDIA
// e2e suites: how many GPUs are on the instance, and what NVIDIA calls
// the GPU model. Both come from ec2:DescribeInstanceTypes' GpuInfo
// field.
//
// Manufacturer is included for callers that want to gate NVIDIA-only
// tests off (e.g. Trainium / Inferentia instances have GpuInfo=nil).
type GPUInfo struct {
	Count        int
	Name         string
	Manufacturer string
}

// GPUInfoForInstanceType looks up the GPU count and product name for an
// EC2 instance type via ec2:DescribeInstanceTypes. When region is
// non-empty it overrides the AWS SDK default chain; empty falls back
// to AWS_REGION env, ~/.aws/config, or IMDS on an EC2 host. Returns an
// error when the instance type has no GPUs or the API call fails.
func GPUInfoForInstanceType(ctx context.Context, instanceType, region string) (GPUInfo, error) {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return GPUInfo{}, fmt.Errorf("load AWS config: %w", err)
	}
	client := ec2.NewFromConfig(cfg)
	out, err := client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
	})
	if err != nil {
		return GPUInfo{}, fmt.Errorf("DescribeInstanceTypes(%s): %w", instanceType, err)
	}
	if len(out.InstanceTypes) == 0 {
		return GPUInfo{}, fmt.Errorf("DescribeInstanceTypes returned no results for %s", instanceType)
	}
	gpuInfo := out.InstanceTypes[0].GpuInfo
	if gpuInfo == nil || len(gpuInfo.Gpus) == 0 {
		return GPUInfo{}, fmt.Errorf("%s has no GPUs (GpuInfo is empty)", instanceType)
	}
	// Assume homogeneous GPUs per instance -- AWS has never mixed SKUs
	// on a single instance type, and the API groups them accordingly.
	g := gpuInfo.Gpus[0]
	info := GPUInfo{}
	if g.Count != nil {
		info.Count = int(*g.Count)
	}
	if g.Name != nil {
		info.Name = *g.Name
	}
	if g.Manufacturer != nil {
		info.Manufacturer = *g.Manufacturer
	}
	return info, nil
}
