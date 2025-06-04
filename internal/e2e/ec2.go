package e2e

import (
	"context"
	"fmt"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2Client interface {
	DescribeInstanceType(instanceType string) (ec2types.InstanceTypeInfo, error)
}

type ec2Client struct {
	client *ec2.Client
}

func NewEC2Client() *ec2Client {
	return &ec2Client{
		client: ec2.NewFromConfig(awssdk.NewConfig()),
	}
}

func (c *ec2Client) DescribeInstanceTopology(instanceIDs []string) ([]ec2types.InstanceTopology, error) {
	var instanceTopologies []ec2types.InstanceTopology
	paginator := ec2.NewDescribeInstanceTopologyPaginator(c.client, &ec2.DescribeInstanceTopologyInput{
		InstanceIds: instanceIDs,
	})
	for paginator.HasMorePages() {
		instanceTopologyOuput, err := paginator.NextPage(context.TODO())
		if err != nil {
			return []ec2types.InstanceTopology{}, err
		}
		instanceTopologies = append(instanceTopologies, instanceTopologyOuput.Instances...)
	}
	return instanceTopologies, nil
}

func (c *ec2Client) DescribeInstanceType(instanceType string) (ec2types.InstanceTypeInfo, error) {
	describeResponse, err := c.client.DescribeInstanceTypes(context.TODO(), &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []ec2types.InstanceType{ec2types.InstanceType(instanceType)},
	})
	if err != nil {
		return ec2types.InstanceTypeInfo{}, fmt.Errorf("failed to describe instance type: %s: %v", instanceType, err)
	} else {
		return describeResponse.InstanceTypes[0], nil
	}
}
