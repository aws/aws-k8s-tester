package eksapi

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"k8s.io/klog/v2"
)

func NewAMIResolver(awsClients *awsClients) *amiResolver {
	return &amiResolver{
		clients: awsClients,
	}
}

type amiResolver struct {
	clients *awsClients
}

func (r *amiResolver) Resolve(ctx context.Context, opts *deployerOptions) (string, error) {
	switch opts.UserDataFormat {
	case UserDataBootstrapSh:
		// TODO: AL2 is not a high priority, skipping for now.
		return "", fmt.Errorf("%s is not handled", opts.UserDataFormat)
	case UserDataNodeadm:
		return r.ResolveAL2023(ctx, opts)
	case UserDataBottlerocket:
		return r.ResolveBottlerocket(ctx, opts)
	default:
		return "", fmt.Errorf("unhandled userdata format: %s", opts.UserDataFormat)
	}
}

func (r *amiResolver) ResolveAL2023(ctx context.Context, opts *deployerOptions) (string, error) {
	describeInstanceTypesResponse, err := r.clients.EC2().DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(r.getInstance(opts))},
	})
	if err != nil {
		return "", err
	}
	instanceTypeInfo := describeInstanceTypesResponse.InstanceTypes[0]

	arch, err := r.resolveArch(instanceTypeInfo)
	if err != nil {
		return "", err
	}

	variant := "standard"
	if instanceTypeInfo.NeuronInfo != nil {
		if len(instanceTypeInfo.NeuronInfo.NeuronDevices) > 0 {
			variant = "neuron"
		}
	} else if instanceTypeInfo.GpuInfo != nil {
		for _, gpu := range instanceTypeInfo.GpuInfo.Gpus {
			if aws.ToString(gpu.Manufacturer) == "NVIDIA" {
				variant = "nvidia"
				break
			}
		}
	}

	getParameterReponse, err := r.clients.SSM().GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf("/aws/service/eks/optimized-ami/%s/amazon-linux-2023/%s/%s/recommended/image_id", opts.KubernetesVersion, arch, variant)),
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(getParameterReponse.Parameter.Value), nil
}

func (r *amiResolver) ResolveBottlerocket(ctx context.Context, opts *deployerOptions) (string, error) {
	describeInstanceTypesResponse, err := r.clients.EC2().DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(r.getInstance(opts))},
	})
	if err != nil {
		return "", err
	}
	instanceTypeInfo := describeInstanceTypesResponse.InstanceTypes[0]

	arch, err := r.resolveArch(instanceTypeInfo)
	if err != nil {
		return "", err
	}

	// TODO: enable fips
	flavorSuffix := ""
	if instanceTypeInfo.GpuInfo != nil {
		for _, gpu := range instanceTypeInfo.GpuInfo.Gpus {
			if aws.ToString(gpu.Manufacturer) == "NVIDIA" {
				flavorSuffix = "-nvidia"
				break
			}
		}
	}

	getParameterResponse, err := r.clients.SSM().GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String(fmt.Sprintf("/aws/service/bottlerocket/aws-k8s-%s%s/%s/latest/image_id", opts.KubernetesVersion, flavorSuffix, arch)),
	})
	if err != nil {
		return "", err
	}

	return aws.ToString(getParameterResponse.Parameter.Value), nil
}

func (r *amiResolver) getInstance(opts *deployerOptions) string {
	instanceType := opts.InstanceTypes[0]
	if len(opts.InstanceTypes) > 1 {
		klog.Warningf("only resolving AMI based on first instance type: %s", instanceType)
	}
	return instanceType
}

func (r *amiResolver) resolveArch(instanceTypeInfo ec2types.InstanceTypeInfo) (string, error) {
	// TODO: the ordering might be weird because old instances might support
	// both i386 and x8664.
	switch arch := instanceTypeInfo.ProcessorInfo.SupportedArchitectures[0]; arch {
	case ec2types.ArchitectureTypeArm64, ec2types.ArchitectureTypeX8664:
		return string(arch), nil
	default:
		return "", fmt.Errorf("unhandled arch: %s", arch)
	}
}
