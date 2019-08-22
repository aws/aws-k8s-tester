package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
)

// EC2 is an wrapper around original EC2API with additional convenient APIs.
type EC2 interface {
	ec2iface.EC2API

	GetSubnetsByNameOrID(ctx context.Context, nameOrIDs []string) ([]*ec2.Subnet, error)
	DescribeSecurityGroupsAsList(ctx context.Context, input *ec2.DescribeSecurityGroupsInput) ([]*ec2.SecurityGroup, error)
	DescribeInstancesAsList(ctx context.Context, input *ec2.DescribeInstancesInput) ([]*ec2.Instance, error)
	WaitForDesiredNetworkInterfaceCount(input *ec2.DescribeNetworkInterfacesInput, count int) error
	WaitForDesiredNetworkInterfaceCountWithContext(ctx aws.Context, input *ec2.DescribeNetworkInterfacesInput, count int, opts ...request.WaiterOption) error
}

func NewEC2(session *session.Session) EC2 {
	return &defaultEC2{
		ec2.New(session),
	}
}

var _ EC2 = (*defaultEC2)(nil)

type defaultEC2 struct {
	ec2iface.EC2API
}

func (c *defaultEC2) GetSubnetsByNameOrID(ctx context.Context, nameOrIDs []string) ([]*ec2.Subnet, error) {
	var names []string
	var ids []string
	for _, s := range nameOrIDs {
		if strings.HasPrefix(s, "subnet-") {
			ids = append(ids, s)
		} else {
			names = append(names, s)
		}
	}

	var filters [][]*ec2.Filter
	if len(ids) > 0 {
		filters = append(filters, []*ec2.Filter{
			{
				Name:   aws.String("subnet-id"),
				Values: aws.StringSlice(ids),
			},
		})
	}
	if len(names) > 0 {
		filters = append(filters, []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice(names),
			},
		})
	}

	var subnets []*ec2.Subnet
	for _, in := range filters {
		describeSubnetsOutput, err := c.DescribeSubnetsWithContext(ctx, &ec2.DescribeSubnetsInput{Filters: in})
		if err != nil {
			return nil, err
		}
		subnets = append(subnets, describeSubnetsOutput.Subnets...)
	}

	return subnets, nil
}

func (c *defaultEC2) DescribeSecurityGroupsAsList(ctx context.Context, input *ec2.DescribeSecurityGroupsInput) ([]*ec2.SecurityGroup, error) {
	var result []*ec2.SecurityGroup
	if err := c.DescribeSecurityGroupsPagesWithContext(ctx, input, func(output *ec2.DescribeSecurityGroupsOutput, _ bool) bool {
		result = append(result, output.SecurityGroups...)
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *defaultEC2) DescribeInstancesAsList(ctx context.Context, input *ec2.DescribeInstancesInput) ([]*ec2.Instance, error) {
	var result []*ec2.Instance
	if err := c.DescribeInstancesPagesWithContext(ctx, input, func(output *ec2.DescribeInstancesOutput, _ bool) bool {
		for _, item := range output.Reservations {
			result = append(result, item.Instances...)
		}
		return true
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// WaitForDesiredNetworkInterfaceCount uses the Amazon EC2 API operation
// DescribeNetworkInterfaces to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *defaultEC2) WaitForDesiredNetworkInterfaceCount(input *ec2.DescribeNetworkInterfacesInput, count int) error {
	return c.WaitForDesiredNetworkInterfaceCountWithContext(aws.BackgroundContext(), input, count)
}

// WaitUntilNetworkInterfaceInUseWithContext is an extended version of WaitForDesiredNetworkInterfaceCount.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *defaultEC2) WaitForDesiredNetworkInterfaceCountWithContext(ctx aws.Context, input *ec2.DescribeNetworkInterfacesInput, count int, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilNetworkInterfaceAvailableOrInUse",
		MaxAttempts: 20,
		Delay:       request.ConstantWaiterDelay(10 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: fmt.Sprintf("length(NetworkInterfaces) == `%d`", count),
				Expected: true,
			},
			{
				State:   request.RetryWaiterState,
				Matcher: request.PathWaiterMatch, Argument: fmt.Sprintf("length(NetworkInterfaces) == `%d`", count),
				Expected: false,
			},
			{
				State:    request.FailureWaiterState,
				Matcher:  request.ErrorWaiterMatch,
				Expected: "InvalidNetworkInterfaceID.NotFound",
			},
		},
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *ec2.DescribeNetworkInterfacesInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeNetworkInterfacesRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}
