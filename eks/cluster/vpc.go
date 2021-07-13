package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	aws_elbv2_v2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

/*
see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/cluster/vpc.go for CloudFormation based workflow

ref.
https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Internet_Gateway.html#vpc-igw-internet-access
https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html
https://amazon-eks.s3.us-west-2.amazonaws.com/cloudformation/2019-11-15/amazon-eks-vpc-private-subnets.yaml
Fargate for EKS does not support public subnet
Customer ENIs attached to Fargate task is not given a Public IP/EIP
Fargate node is not assigned a PublicIP/EIP
EC2::InternetGateway
EC2::InternetGateway allows "inbound" and "outbound" traffic from/to the internet
EC2::InternetGateway does NAT (network address translation) between two IP addresses:
- Public IP/EIP assigned to the EC2 instance
- Private IP assigned to the EC2 instance
EC2::NatGateway
EC2::NatGateway only allows "outbound" traffic to the internet
Cannot SSH into an instance with a public IP but private subnet
since EC2::NatGateway only allows "outbound" traffic
network address translation (NAT) gateway in the specified public subnet
NAT gateway to allow instances in a private subnet to connect to the Internet
or to other AWS services, but prevent the Internet from initiating a connection with those instances.
Public Subnet
Subnet associated with EC2::RouteTable that has EC2::Route to an EC2::InternetGateway
  PublicRoute:
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref PublicRouteTable
      DestinationCidrBlock: 0.0.0.0/0
      GatewayId: !Ref InternetGateway
Private Subnet
Subnet associated with EC2::RouteTable that has EC2::Route to an EC2::NatGateway
  PrivateRoute1:
    Type: AWS::EC2::Route
    Properties:
      RouteTableId: !Ref PrivateRouteTable1
      DestinationCidrBlock: 0.0.0.0/0
      NatGatewayId: !Ref NATGateway1
*/

func (ts *tester) createVPC() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createVPC [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if ts.cfg.EKSConfig.VPC.ID != "" {
		ts.cfg.Logger.Info("querying ELBv2", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := ts.cfg.ELBV2APIV2.DescribeLoadBalancers(
			ctx,
			&aws_elbv2_v2.DescribeLoadBalancersInput{},
		)
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to describe ELBv2", zap.Error(err))
		} else {
			for _, ev := range output.LoadBalancers {
				arn := aws_v2.ToString(ev.LoadBalancerArn)
				vpcID := aws_v2.ToString(ev.VpcId)
				if vpcID == ts.cfg.EKSConfig.VPC.ID {
					ts.cfg.Logger.Warn("found ELBv2 for this VPC; may overlap with the other cluster",
						zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID),
						zap.String("elb-arn", arn),
					)
				} else {
					ts.cfg.Logger.Info("found ELBv2 for other VPCs", zap.String("vpc-id", vpcID), zap.String("elb-arn", arn))
				}
			}
		}

		ts.cfg.Logger.Info("querying subnet IDs for given VPC",
			zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID),
		)
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		sresp, err := ts.cfg.EC2APIV2.DescribeSubnets(
			ctx,
			&aws_ec2_v2.DescribeSubnetsInput{
				Filters: []aws_ec2_v2_types.Filter{
					{
						Name:   aws_v2.String("vpc-id"),
						Values: []string{ts.cfg.EKSConfig.VPC.ID},
					},
				},
			})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to subnets", zap.Error(err))
			return err
		}

		ts.cfg.EKSConfig.VPC.PublicSubnetIDs = make([]string, 0, len(sresp.Subnets))
		ts.cfg.EKSConfig.VPC.PrivateSubnetIDs = make([]string, 0, len(sresp.Subnets))
		for _, sv := range sresp.Subnets {
			id := aws_v2.ToString(sv.SubnetId)
			networkTagValue := ""
			for _, tg := range sv.Tags {
				switch aws_v2.ToString(tg.Key) {
				case "Network":
					networkTagValue = aws_v2.ToString(tg.Value)
				}
				if networkTagValue != "" {
					break
				}
			}
			ts.cfg.Logger.Info("found subnet",
				zap.String("id", id),
				zap.String("availability-zone", aws_v2.ToString(sv.AvailabilityZone)),
				zap.String("network-tag", networkTagValue),
			)
			switch networkTagValue {
			case "Public":
				ts.cfg.EKSConfig.VPC.PublicSubnetIDs = append(ts.cfg.EKSConfig.VPC.PublicSubnetIDs, id)
			case "Private":
				ts.cfg.EKSConfig.VPC.PrivateSubnetIDs = append(ts.cfg.EKSConfig.VPC.PrivateSubnetIDs, id)
			default:
				return fmt.Errorf("'Network' tag not found in subnet %q", id)
			}
		}
		if len(ts.cfg.EKSConfig.VPC.PublicSubnetIDs) == 0 {
			return fmt.Errorf("no subnet found for VPC ID %q", ts.cfg.EKSConfig.VPC.ID)
		}

		ts.cfg.Logger.Info("querying security IDs", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		gresp, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
			ctx,
			&aws_ec2_v2.DescribeSecurityGroupsInput{
				Filters: []aws_ec2_v2_types.Filter{
					{
						Name:   aws_v2.String("vpc-id"),
						Values: []string{ts.cfg.EKSConfig.VPC.ID},
					},
				},
			})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to security groups", zap.Error(err))
			return err
		}
		for _, sg := range gresp.SecurityGroups {
			id, name := aws_v2.ToString(sg.GroupId), aws_v2.ToString(sg.GroupName)
			ts.cfg.Logger.Info("found security group", zap.String("id", id), zap.String("name", name))
			if name != "default" {
				ts.cfg.EKSConfig.VPC.SecurityGroupID = id
			}
		}
		if ts.cfg.EKSConfig.VPC.SecurityGroupID == "" {
			return fmt.Errorf("no security group found for VPC ID %q", ts.cfg.EKSConfig.VPC.ID)
		}

		ts.cfg.EKSConfig.Sync()
		return nil
	}
	if !ts.cfg.EKSConfig.VPC.Create {
		ts.cfg.Logger.Info("VPC.Create false; skipping creation")
		return nil
	}
	if ts.cfg.EKSConfig.VPC.ID != "" &&
		len(ts.cfg.EKSConfig.VPC.PublicSubnetIDs) > 0 &&
		ts.cfg.EKSConfig.VPC.SecurityGroupID != "" {
		ts.cfg.Logger.Info("VPC already created; no need to create a new one")
		return nil
	}

	if err := ts._createVPC(); err != nil { // AWS::EC2::VPC
		return err
	}
	if err := ts.createSecurityGroup(); err != nil { // AWS::EC2::SecurityGroup
		return err
	}
	if err := ts.associateVPCCIDRBlocks(); err != nil { // AWS::EC2::VPCCidrBlock
		return err
	}

	if err := ts.createInternetGateway(); err != nil { // AWS::EC2::InternetGateway
		return err
	}
	if err := ts.createVPCGatewayAttachment(); err != nil { // AWS::EC2::VPCGatewayAttachment
		return err
	}

	if err := ts.createPublicSubnets(); err != nil { // AWS::EC2::Subnet
		return err
	}
	if err := ts.createPublicRouteTable(); err != nil { // AWS::EC2::RouteTable
		return err
	}
	if err := ts.createPublicRoute(); err != nil { // AWS::EC2::Route
		return err
	}
	if err := ts.createPublicSubnetRouteTableAssociation(); err != nil { // AWS::EC2::SubnetRouteTableAssociation
		return err
	}

	if err := ts.createPublicEIPs(); err != nil { // AWS::EC2::EIP
		return err
	}
	if err := ts.createPublicNATGateways(); err != nil { // AWS::EC2::NatGateway
		return err
	}

	if err := ts.createPrivateSubnets(); err != nil { // AWS::EC2::Subnet
		return err
	}
	if err := ts.createPrivateRouteTables(); err != nil { // AWS::EC2::RouteTable
		return err
	}
	if err := ts.createPrivateRoutes(); err != nil { // AWS::EC2::Route
		return err
	}
	if err := ts.createPrivateSubnetRouteTableAssociation(); err != nil { // AWS::EC2::SubnetRouteTableAssociation
		return err
	}

	if err := ts.createDHCPOptions(); err != nil { // AWS::EC2::DHCPOptions, AWS::EC2::VPCDHCPOptionsAssociation
		return err
	}

	ts.cfg.Logger.Info("created a VPC",
		zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID),
		zap.Strings("vpc-cidr-blocks", ts.cfg.EKSConfig.VPC.CIDRs),
		zap.Strings("public-subnet-ids", ts.cfg.EKSConfig.VPC.PublicSubnetIDs),
		zap.Strings("private-subnet-ids", ts.cfg.EKSConfig.VPC.PrivateSubnetIDs),
		zap.String("control-plane-security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID),
	)

	ts.cfg.EKSConfig.Sync()
	return nil
}

// e.g. DependencyViolation: The vpc 'vpc-0127f6d18bd98836a' has dependencies and cannot be deleted
func (ts *tester) deleteVPC() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_blue]deleteVPC [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if !ts.cfg.EKSConfig.VPC.Create {
		ts.cfg.Logger.Info("VPC.Create false; skipping deletion")
		return nil
	}

	waitDur := 30 * time.Second
	ts.cfg.Logger.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	ts.cfg.Logger.Info("deleting VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))

	if ok := ts.deleteELBv2(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteDHCPOptions(); ok {
		time.Sleep(10 * time.Second)
	}

	if ok := ts.deletePrivateSubnetRouteTableAssociation(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePrivateRouteTables(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePrivateSubnets(); ok {
		time.Sleep(10 * time.Second)
	}

	if ok := ts.deleteELBv2(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteENIs(); ok {
		time.Sleep(10 * time.Second)
	}

	if ok := ts.deletePublicNATGateways(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePublicEIPs(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePublicSubnetRouteTableAssociation(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePublicRouteTable(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePrivateSubnets(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deletePublicSubnets(); ok {
		time.Sleep(10 * time.Second)
	}

	if ok := ts.deleteVPCGatewayAttachment(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteInternetGateway(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteSGs(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteSecurityGroup(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts.deleteENIs(); ok {
		time.Sleep(10 * time.Second)
	}
	if ok := ts._deleteVPC(); ok {
		time.Sleep(10 * time.Second)
	}

	return nil
}

// AWS::EC2::VPC
func (ts *tester) _createVPC() error {
	ts.cfg.Logger.Info("creating a VPC", zap.String("cidr-block", ts.cfg.EKSConfig.VPC.CIDRs[0]))
	vpcOut, err := ts.cfg.EC2APIV2.CreateVpc(
		context.Background(),
		&aws_ec2_v2.CreateVpcInput{
			CidrBlock: aws_v2.String(ts.cfg.EKSConfig.VPC.CIDRs[0]),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create a VPC", zap.Error(err))
		return err
	}

	_, err = ts.cfg.EC2APIV2.ModifyVpcAttribute(
		context.Background(),
		&aws_ec2_v2.ModifyVpcAttributeInput{
			EnableDnsHostnames: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
			EnableDnsSupport:   &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to modify VPC attribute", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.ID = aws_v2.ToString(vpcOut.Vpc.VpcId)
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created a VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	return nil
}

func (ts *tester) _deleteVPC() (deleted bool) {
	ts.cfg.Logger.Info("deleting VPC")
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.ID]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DeleteVpc(
		context.Background(),
		&aws_ec2_v2.DeleteVpcInput{
			VpcId: aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to detach VPC", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.ID] = "VPC.ID"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted VPC")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.ID] = "VPC.ID"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

// AWS::EC2::SecurityGroup
func (ts *tester) createSecurityGroup() error {
	ts.cfg.Logger.Info("creating security group")

	sout, err := ts.cfg.EC2APIV2.CreateSecurityGroup(
		context.Background(),
		&aws_ec2_v2.CreateSecurityGroupInput{
			VpcId: aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create security group", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.SecurityGroupID = aws_v2.ToString(sout.GroupId)
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created security group", zap.String("security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID))
	return nil
}

func (ts *tester) deleteSecurityGroup() (deleted bool) {
	ts.cfg.Logger.Info("deleting security group")
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DeleteSecurityGroup(
		context.Background(),
		&aws_ec2_v2.DeleteSecurityGroupInput{
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to detach security group", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted security group")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.SecurityGroupID] = "VPC.SecurityGroupID"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

// AWS::EC2::VPCCidrBlock
func (ts *tester) associateVPCCIDRBlocks() error {
	ts.cfg.Logger.Info("associating VPC CIDR blocks with the rest")
	for _, cidr := range ts.cfg.EKSConfig.VPC.CIDRs[1:] {
		_, err := ts.cfg.EC2APIV2.AssociateVpcCidrBlock(
			context.Background(),
			&aws_ec2_v2.AssociateVpcCidrBlockInput{
				VpcId:     aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
				CidrBlock: aws_v2.String(cidr),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to associate VPC CIDR block", zap.String("cidr-block", cidr), zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("associated VPC CIDR block", zap.String("cidr-block", cidr))
	}
	return nil
}

// AWS::EC2::InternetGateway
func (ts *tester) createInternetGateway() error {
	ts.cfg.Logger.Info("creating internet gateway")
	out, err := ts.cfg.EC2APIV2.CreateInternetGateway(
		context.Background(),
		&aws_ec2_v2.CreateInternetGatewayInput{},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create internet gateway", zap.Error(err))
		return err
	}
	ts.cfg.EKSConfig.VPC.InternetGatewayID = aws_v2.ToString(out.InternetGateway.InternetGatewayId)
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created internet gateway", zap.String("internet-gateway-id", ts.cfg.EKSConfig.VPC.InternetGatewayID))
	return nil
}

func (ts *tester) deleteInternetGateway() (deleted bool) {
	ts.cfg.Logger.Info("deleting internet gateway")
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DeleteInternetGateway(
		context.Background(),
		&aws_ec2_v2.DeleteInternetGatewayInput{
			InternetGatewayId: aws_v2.String(ts.cfg.EKSConfig.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to detach internet gateway", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID] = "VPC.InternetGatewayID"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted internet gateway")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID] = "VPC.InternetGatewayID"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

// AWS::EC2::VPCGatewayAttachment
func (ts *tester) createVPCGatewayAttachment() error {
	ts.cfg.Logger.Info("creating VPC gateway attachment")
	_, err := ts.cfg.EC2APIV2.AttachInternetGateway(
		context.Background(),
		&aws_ec2_v2.AttachInternetGatewayInput{
			VpcId:             aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
			InternetGatewayId: aws_v2.String(ts.cfg.EKSConfig.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create VPC gateway attachment", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("created VPC gateway attachment")
	return nil
}

func (ts *tester) deleteVPCGatewayAttachment() (deleted bool) {
	ts.cfg.Logger.Info("deleting VPC gateway attachment")
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID+"-detach"]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DetachInternetGateway(
		context.Background(),
		&aws_ec2_v2.DetachInternetGatewayInput{
			VpcId:             aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
			InternetGatewayId: aws_v2.String(ts.cfg.EKSConfig.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to detach VPC gateway attachment", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID+"-detach"] = "VPC.InternetGatewayID.detach"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted VPC gateway attachment")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.InternetGatewayID+"-detach"] = "VPC.InternetGatewayID.detach"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

// AWS::EC2::Subnet
func (ts *tester) createPublicSubnets() error {
	ts.cfg.Logger.Info("creating public subnets", zap.Strings("availability-zones", ts.cfg.EKSConfig.AvailabilityZoneNames))

	cidrs := make([]string, len(ts.cfg.EKSConfig.AvailabilityZoneNames))
	copy(cidrs, ts.cfg.EKSConfig.VPC.PublicSubnetCIDRs)

	ts.cfg.EKSConfig.VPC.PublicSubnetIDs = make([]string, 0)
	for idx, cidr := range cidrs {
		sout, err := ts.cfg.EC2APIV2.CreateSubnet(
			context.Background(),
			&aws_ec2_v2.CreateSubnetInput{
				VpcId:            aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
				AvailabilityZone: aws_v2.String(ts.cfg.EKSConfig.AvailabilityZoneNames[idx]),
				CidrBlock:        aws_v2.String(cidr),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-public-subnet-%d", ts.cfg.EKSConfig.Name, idx+1)),
							},
							{
								Key:   aws_v2.String("Network"),
								Value: aws_v2.String("Public"),
							},
							{
								// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.htmls
								Key:   aws_v2.String("kubernetes.io/role/elb"),
								Value: aws_v2.String("1"),
							},
							{
								// https://docs.aws.amazon.com/eks/latest/userguide/alb-ingress.htmls
								Key:   aws_v2.String("kubernetes.io/role/internal-elb"),
								Value: aws_v2.String("1"),
							},
						},
					},
				},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create public subnets", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		subnetID := aws_v2.ToString(sout.Subnet.SubnetId)
		ts.cfg.EKSConfig.VPC.PublicSubnetIDs = append(ts.cfg.EKSConfig.VPC.PublicSubnetIDs, subnetID)
		ts.cfg.Logger.Info("created a public subnet", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))

		_, err = ts.cfg.EC2APIV2.ModifySubnetAttribute(
			context.Background(),
			&aws_ec2_v2.ModifySubnetAttributeInput{
				SubnetId:            sout.Subnet.SubnetId,
				MapPublicIpOnLaunch: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to modify subnet attribute", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("modified the public subnet with MapPublicIpOnLaunch", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created public subnets", zap.Strings("availability-zones", ts.cfg.EKSConfig.AvailabilityZoneNames))
	return nil
}

func (ts *tester) deletePublicSubnets() (deleted bool) {
	ts.cfg.Logger.Info("deleting public subnets")

	deleted = true
	for _, subnet := range ts.cfg.EKSConfig.VPC.PublicSubnetIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[subnet]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DeleteSubnet(
			context.Background(),
			&aws_ec2_v2.DeleteSubnetInput{
				SubnetId: aws_v2.String(subnet),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete public subnet", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[subnet] = "VPC.PublicSubnetID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[subnet] = "VPC.PublicSubnetID"
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("deleted public subnets")
	return deleted
}

// AWS::EC2::RouteTable
func (ts *tester) createPublicRouteTable() error {
	ts.cfg.Logger.Info("creating public route table")

	out, err := ts.cfg.EC2APIV2.CreateRouteTable(
		context.Background(),
		&aws_ec2_v2.CreateRouteTableInput{
			VpcId: aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
			TagSpecifications: []aws_ec2_v2_types.TagSpecification{
				{
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String("Name"),
							Value: aws_v2.String(fmt.Sprintf("%s-public-route-table", ts.cfg.EKSConfig.Name)),
						},
						{
							Key:   aws_v2.String("Network"),
							Value: aws_v2.String("Public"),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create public route table", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.PublicRouteTableID = aws_v2.ToString(out.RouteTable.RouteTableId)
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created public route table", zap.String("route-table-id", ts.cfg.EKSConfig.VPC.PublicRouteTableID))
	return nil
}

func (ts *tester) deletePublicRouteTable() (deleted bool) {
	ts.cfg.Logger.Info("deleting public route table")
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.PublicRouteTableID]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DeleteRouteTable(
		context.Background(),
		&aws_ec2_v2.DeleteRouteTableInput{
			RouteTableId: aws_v2.String(ts.cfg.EKSConfig.VPC.PublicRouteTableID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to delete public route table", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.PublicRouteTableID] = "VPC.PublicRouteTableID"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted public route table")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.PublicRouteTableID] = "VPC.PublicRouteTableID"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

// AWS::EC2::Route
func (ts *tester) createPublicRoute() error {
	ts.cfg.Logger.Info("creating public route")

	_, err := ts.cfg.EC2APIV2.CreateRoute(
		context.Background(),
		&aws_ec2_v2.CreateRouteInput{
			RouteTableId:         aws_v2.String(ts.cfg.EKSConfig.VPC.PublicRouteTableID),
			GatewayId:            aws_v2.String(ts.cfg.EKSConfig.VPC.InternetGatewayID),
			DestinationCidrBlock: aws_v2.String("0.0.0.0/0"),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create public route table", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("created public route")
	return nil
}

// AWS::EC2::SubnetRouteTableAssociation
func (ts *tester) createPublicSubnetRouteTableAssociation() error {
	ts.cfg.Logger.Info("creating public subnet route table association")

	for _, subnet := range ts.cfg.EKSConfig.VPC.PublicSubnetIDs {
		out, err := ts.cfg.EC2APIV2.AssociateRouteTable(
			context.Background(),
			&aws_ec2_v2.AssociateRouteTableInput{
				SubnetId:     aws_v2.String(subnet),
				RouteTableId: aws_v2.String(ts.cfg.EKSConfig.VPC.PublicRouteTableID),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to associate route table", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.VPC.PublicSubnetRouteTableAssociationIDs = append(ts.cfg.EKSConfig.VPC.PublicSubnetRouteTableAssociationIDs, aws_v2.ToString(out.AssociationId))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created public subnet route table association")
	return nil
}

func (ts *tester) deletePublicSubnetRouteTableAssociation() (deleted bool) {
	ts.cfg.Logger.Info("deleting public subnet route table association")

	deleted = true
	for _, id := range ts.cfg.EKSConfig.VPC.PublicSubnetRouteTableAssociationIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DisassociateRouteTable(
			context.Background(),
			&aws_ec2_v2.DisassociateRouteTableInput{
				AssociationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to disassociate route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.PublicSubnetRouteTableAssociationID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.PublicSubnetRouteTableAssociationID"
		ts.cfg.EKSConfig.Sync()
	}

	if deleted {
		ts.cfg.Logger.Info("deleted public subnet route table association")
	}
	return deleted
}

// AWS::EC2::EIP
func (ts *tester) createPublicEIPs() error {
	ts.cfg.Logger.Info("creating public EIPs")

	for idx := range ts.cfg.EKSConfig.VPC.PublicSubnetIDs {
		// tags are not supported in ISO regions
		tags := make([]aws_ec2_v2_types.TagSpecification, 0)
		if !strings.Contains(ts.cfg.EKSConfig.Partition, "-iso") {
			tags = []aws_ec2_v2_types.TagSpecification{
				{
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String("Name"),
							Value: aws_v2.String(fmt.Sprintf("%s-eip-%d", ts.cfg.EKSConfig.Name, idx+1)),
						},
					},
				},
			}
		}
		out, err := ts.cfg.EC2APIV2.AllocateAddress(
			context.Background(),
			&aws_ec2_v2.AllocateAddressInput{
				Domain:            aws_ec2_v2_types.DomainTypeVpc,
				TagSpecifications: tags,
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create EIP", zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("created EIP",
			zap.String("public-ip", aws_v2.ToString(out.PublicIp)),
			zap.String("allocation-id", aws_v2.ToString(out.AllocationId)),
		)
		ts.cfg.EKSConfig.VPC.EIPAllocationIDs = append(ts.cfg.EKSConfig.VPC.EIPAllocationIDs, aws_v2.ToString(out.AllocationId))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created public EIPs")
	return nil
}

func (ts *tester) deletePublicEIPs() (deleted bool) {
	ts.cfg.Logger.Info("deleting public EIPs")

	deleted = true
	for _, id := range ts.cfg.EKSConfig.VPC.EIPAllocationIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.ReleaseAddress(
			context.Background(),
			&aws_ec2_v2.ReleaseAddressInput{
				AllocationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete EIP", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.EIPAllocationID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.EIPAllocationID"
		ts.cfg.EKSConfig.Sync()
	}

	if deleted {
		ts.cfg.Logger.Info("deleted public EIPs")
	}
	return deleted
}

// AWS::EC2::NatGateway
func (ts *tester) createPublicNATGateways() error {
	ts.cfg.Logger.Info("creating public NAT gateways using EIPs and public subnets")

	for idx, subnet := range ts.cfg.EKSConfig.VPC.PublicSubnetIDs {
		out, err := ts.cfg.EC2APIV2.CreateNatGateway(
			context.Background(),
			&aws_ec2_v2.CreateNatGatewayInput{
				SubnetId:     aws_v2.String(subnet),
				AllocationId: aws_v2.String(ts.cfg.EKSConfig.VPC.EIPAllocationIDs[idx]),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create NAT gateway", zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("created NAT gateway", zap.String("nat-gateway-id", aws_v2.ToString(out.NatGateway.NatGatewayId)))
		ts.cfg.EKSConfig.VPC.NATGatewayIDs = append(ts.cfg.EKSConfig.VPC.NATGatewayIDs, aws_v2.ToString(out.NatGateway.NatGatewayId))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created public NAT gateways")
	return nil
}

func (ts *tester) deletePublicNATGateways() (deleted bool) {
	ts.cfg.Logger.Info("deleting public NAT gateways")

	deleted = true
	for _, id := range ts.cfg.EKSConfig.VPC.NATGatewayIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DeleteNatGateway(
			context.Background(),
			&aws_ec2_v2.DeleteNatGatewayInput{
				NatGatewayId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete public NAT gateway", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.NATGatewayID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.NATGatewayID"
		ts.cfg.EKSConfig.Sync()
	}
	ts.cfg.EKSConfig.Sync()

	if deleted {
		ts.cfg.Logger.Info("deleted public NAT gateways")
	}
	return deleted
}

// AWS::EC2::Subnet
func (ts *tester) createPrivateSubnets() error {
	ts.cfg.Logger.Info("creating private subnets", zap.Strings("availability-zones", ts.cfg.EKSConfig.AvailabilityZoneNames))

	ts.cfg.EKSConfig.VPC.PrivateSubnetIDs = make([]string, 0)
	for idx, cidr := range ts.cfg.EKSConfig.VPC.PrivateSubnetCIDRs {
		sout, err := ts.cfg.EC2APIV2.CreateSubnet(
			context.Background(),
			&aws_ec2_v2.CreateSubnetInput{
				VpcId:            aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
				AvailabilityZone: aws_v2.String(ts.cfg.EKSConfig.AvailabilityZoneNames[idx]),
				CidrBlock:        aws_v2.String(cidr),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-private-subnet-%d", ts.cfg.EKSConfig.Name, idx+1)),
							},
							{
								Key:   aws_v2.String("Network"),
								Value: aws_v2.String("Private"),
							},
						},
					},
				},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create private subnets", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		subnetID := aws_v2.ToString(sout.Subnet.SubnetId)
		ts.cfg.EKSConfig.VPC.PrivateSubnetIDs = append(ts.cfg.EKSConfig.VPC.PrivateSubnetIDs, subnetID)
		ts.cfg.Logger.Info("created a private subnet", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))

		_, err = ts.cfg.EC2APIV2.ModifySubnetAttribute(
			context.Background(),
			&aws_ec2_v2.ModifySubnetAttributeInput{
				SubnetId:            sout.Subnet.SubnetId,
				MapPublicIpOnLaunch: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(false)},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to modify subnet attribute", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("modified the private subnet with MapPublicIpOnLaunch", zap.String("availability-zone", ts.cfg.EKSConfig.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created private subnets", zap.Strings("availability-zones", ts.cfg.EKSConfig.AvailabilityZoneNames))
	return nil
}

func (ts *tester) deletePrivateSubnets() (deleted bool) {
	ts.cfg.Logger.Info("deleting private subnets")

	deleted = true
	for _, subnet := range ts.cfg.EKSConfig.VPC.PrivateSubnetIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[subnet]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DeleteSubnet(
			context.Background(),
			&aws_ec2_v2.DeleteSubnetInput{
				SubnetId: aws_v2.String(subnet),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete private subnet", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[subnet] = "VPC.PrivateSubnetID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[subnet] = "VPC.PrivateSubnetID"
		ts.cfg.EKSConfig.Sync()
	}

	if deleted {
		ts.cfg.Logger.Info("deleted private subnets")
	}
	return deleted
}

// AWS::EC2::RouteTable
func (ts *tester) createPrivateRouteTables() error {
	ts.cfg.Logger.Info("creating private route tables using VPC")

	for idx := range ts.cfg.EKSConfig.VPC.PrivateSubnetIDs {
		out, err := ts.cfg.EC2APIV2.CreateRouteTable(
			context.Background(),
			&aws_ec2_v2.CreateRouteTableInput{
				VpcId: aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-private-route-table-%d", ts.cfg.EKSConfig.Name, idx+1)),
							},
							{
								Key:   aws_v2.String("Network"),
								Value: aws_v2.String("private"),
							},
						},
					},
				},
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create private route table", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs = append(ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs, aws_v2.ToString(out.RouteTable.RouteTableId))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created private route tables", zap.Strings("route-table-ids", ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs))
	return nil
}

func (ts *tester) deletePrivateRouteTables() (deleted bool) {
	ts.cfg.Logger.Info("deleting private route tables")

	deleted = true
	for _, routeTableID := range ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[routeTableID]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DeleteRouteTable(
			context.Background(),
			&aws_ec2_v2.DeleteRouteTableInput{
				RouteTableId: aws_v2.String(routeTableID),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete private route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[routeTableID] = "VPC.PrivateRouteTableID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[routeTableID] = "VPC.PrivateRouteTableID"
		ts.cfg.EKSConfig.Sync()
	}

	if deleted {
		ts.cfg.Logger.Info("deleted private route tables")
	}
	return deleted
}

// AWS::EC2::Route
func (ts *tester) createPrivateRoutes() error {
	ts.cfg.Logger.Info("creating private routes using NAT gateways and private route tables")

	// we create 3 public subnets and 3 NAT gateways for 3-AZ regions
	// the last NAT gateway may be unmapped
	for idx, route := range ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs {
		_, err := ts.cfg.EC2APIV2.CreateRoute(
			context.Background(),
			&aws_ec2_v2.CreateRouteInput{
				RouteTableId:         aws_v2.String(route),
				GatewayId:            aws_v2.String(ts.cfg.EKSConfig.VPC.NATGatewayIDs[idx]),
				DestinationCidrBlock: aws_v2.String("0.0.0.0/0"),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create private route table", zap.Error(err))
			return err
		}
	}

	ts.cfg.Logger.Info("created private routes")
	return nil
}

// AWS::EC2::SubnetRouteTableAssociation
func (ts *tester) createPrivateSubnetRouteTableAssociation() error {
	ts.cfg.Logger.Info("creating private subnet route table association using private route table")

	for idx, subnet := range ts.cfg.EKSConfig.VPC.PrivateSubnetIDs {
		out, err := ts.cfg.EC2APIV2.AssociateRouteTable(
			context.Background(),
			&aws_ec2_v2.AssociateRouteTableInput{
				SubnetId:     aws_v2.String(subnet),
				RouteTableId: aws_v2.String(ts.cfg.EKSConfig.VPC.PrivateRouteTableIDs[idx]),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to associate route table", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.VPC.PrivateSubnetRouteTableAssociationIDs = append(ts.cfg.EKSConfig.VPC.PrivateSubnetRouteTableAssociationIDs, aws_v2.ToString(out.AssociationId))
	}
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("created private subnet route table association")
	return nil
}

func (ts *tester) deletePrivateSubnetRouteTableAssociation() (deleted bool) {
	ts.cfg.Logger.Info("deleting private subnet route table association")

	deleted = true
	for _, id := range ts.cfg.EKSConfig.VPC.PrivateSubnetRouteTableAssociationIDs {
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.cfg.EC2APIV2.DisassociateRouteTable(
			context.Background(),
			&aws_ec2_v2.DisassociateRouteTableInput{
				AssociationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.cfg.Logger.Warn("failed to disassociate route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.PrivateSubnetRouteTableAssociationID"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.EKSConfig.Status.DeletedResources[id] = "VPC.PrivateSubnetRouteTableAssociationID"
		ts.cfg.EKSConfig.Sync()
	}

	if deleted {
		ts.cfg.Logger.Info("deleted private subnet route table association")
	}
	return deleted
}

// AWS::EC2::DHCPOptions
// AWS::EC2::VPCDHCPOptionsAssociation
func (ts *tester) createDHCPOptions() error {
	ts.cfg.Logger.Info("creating DHCP options domain name and servers")
	if len(ts.cfg.EKSConfig.VPC.DHCPOptionsDomainName) == 0 && len(ts.cfg.EKSConfig.VPC.DHCPOptionsDomainNameServers) == 0 {
		return nil
	}

	dhcpConfigs := make([]aws_ec2_v2_types.NewDhcpConfiguration, 0)
	if ts.cfg.EKSConfig.VPC.DHCPOptionsDomainName != "" {
		dhcpConfigs = append(dhcpConfigs, aws_ec2_v2_types.NewDhcpConfiguration{
			Key:    aws_v2.String("DomainName"),
			Values: []string{ts.cfg.EKSConfig.VPC.DHCPOptionsDomainName},
		})
	}
	if len(ts.cfg.EKSConfig.VPC.DHCPOptionsDomainNameServers) > 0 {
		dhcpConfigs = append(dhcpConfigs, aws_ec2_v2_types.NewDhcpConfiguration{
			Key:    aws_v2.String("DomainNameServers"),
			Values: ts.cfg.EKSConfig.VPC.DHCPOptionsDomainNameServers,
		})
	}

	dhcpOut, err := ts.cfg.EC2APIV2.CreateDhcpOptions(
		context.Background(),
		&aws_ec2_v2.CreateDhcpOptionsInput{
			DhcpConfigurations: dhcpConfigs,
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to associate DHCP options", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.DHCPOptionsID = aws_v2.ToString(dhcpOut.DhcpOptions.DhcpOptionsId)
	ts.cfg.EKSConfig.Sync()

	_, err = ts.cfg.EC2APIV2.AssociateDhcpOptions(
		context.Background(),
		&aws_ec2_v2.AssociateDhcpOptionsInput{
			DhcpOptionsId: dhcpOut.DhcpOptions.DhcpOptionsId,
			VpcId:         aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to associate DHCP options", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("created and associated DHCP options domain name and servers")
	return nil
}

func (ts *tester) deleteDHCPOptions() (deleted bool) {
	ts.cfg.Logger.Info("deleting DHCP options domain name and servers")
	if len(ts.cfg.EKSConfig.VPC.DHCPOptionsDomainName) == 0 && len(ts.cfg.EKSConfig.VPC.DHCPOptionsDomainNameServers) == 0 {
		return true
	}
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.DHCPOptionsID]; ok {
		return true
	}

	deleted = true
	_, err := ts.cfg.EC2APIV2.DeleteDhcpOptions(
		context.Background(),
		&aws_ec2_v2.DeleteDhcpOptionsInput{
			DhcpOptionsId: aws_v2.String(ts.cfg.EKSConfig.VPC.DHCPOptionsID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to delete DHCP options", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.DHCPOptionsID] = "VPC.DHCPOptionsID"
				ts.cfg.EKSConfig.Sync()
			} else {
				deleted = false
			}
		} else {
			deleted = false
		}
		return deleted
	}

	ts.cfg.Logger.Info("deleted and disassociated DHCP options domain name and servers")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.DHCPOptionsID] = "VPC.DHCPOptionsID"
	ts.cfg.EKSConfig.Sync()
	return deleted
}

func (ts *tester) deleteELBv2() (deleted bool) {
	ts.cfg.Logger.Info("deleting ELBv2 for the VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	elbARNs := make([]string, 0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	output, err := ts.cfg.ELBV2APIV2.DescribeLoadBalancers(
		ctx,
		&aws_elbv2_v2.DescribeLoadBalancersInput{},
	)
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ELBv2", zap.Error(err))
		return false
	}
	if len(output.LoadBalancers) == 0 {
		ts.cfg.Logger.Info("ELBv2 not found")
		return true
	}

	for _, ev := range output.LoadBalancers {
		vpcID := aws_v2.ToString(ev.VpcId)
		elbV2ARN := aws_v2.ToString(ev.LoadBalancerArn)
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[elbV2ARN]; ok {
			continue
		}
		if vpcID != ts.cfg.EKSConfig.VPC.ID {
			ts.cfg.Logger.Info("found ELBv2 for other VPCs",
				zap.String("vpc-id", vpcID),
				zap.String("elb-v2-arn", elbV2ARN),
			)
			continue
		}
		elbARNs = append(elbARNs, elbV2ARN)
		ts.cfg.Logger.Info("found ELBv2 for this VPC",
			zap.String("vpc-id", vpcID),
			zap.String("elb-v2-arn", elbV2ARN),
		)
	}

	deleted = true
	for _, arn := range elbARNs {
		ts.cfg.Logger.Info("removing ELBv2",
			zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID),
			zap.String("elb-v2-arn", arn),
		)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		_, err = ts.cfg.ELBV2APIV2.DeleteLoadBalancer(
			ctx,
			&aws_elbv2_v2.DeleteLoadBalancerInput{
				LoadBalancerArn: aws_v2.String(arn),
			})
		cancel()

		if err != nil {
			ts.cfg.Logger.Warn("failed to remove ELBv2",
				zap.String("elb-v2-arn", arn),
				zap.Error(err),
			)
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.EKSConfig.Status.DeletedResources[arn] = "ELBV2"
					ts.cfg.EKSConfig.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}

		ts.cfg.Logger.Info("removed ELBv2", zap.String("elb-arn", arn), zap.Error(err))
		ts.cfg.EKSConfig.Status.DeletedResources[arn] = "ELBV2"
	}

	ts.cfg.Logger.Info("deleted ELBv2 for the VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	return deleted
}

func (ts *tester) deleteENIs() bool {
	ts.cfg.Logger.Info("deleting ENIs for the VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	enis := make([]aws_ec2_v2_types.NetworkInterface, 0)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	out, err := ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
		ctx,
		&aws_ec2_v2.DescribeNetworkInterfacesInput{
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("vpc-id"),
					Values: []string{ts.cfg.EKSConfig.VPC.ID},
				},
			},
		},
	)
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe ENIs", zap.Error(err))
		return false
	}
	for _, eni := range out.NetworkInterfaces {
		eniID := aws_v2.ToString(eni.NetworkInterfaceId)
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[eniID]; ok {
			continue
		}
		enis = append(enis, eni)
		ts.cfg.Logger.Info("found ENI", zap.String("eni", eniID))
	}

	// detacth and delete ENIs
	deleted := false
	for _, eni := range enis {
		eniID := aws_v2.ToString(eni.NetworkInterfaceId)

		ts.cfg.Logger.Warn("detaching ENI", zap.String("eni", eniID))
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		out, err := ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
			ctx,
			&aws_ec2_v2.DescribeNetworkInterfacesInput{
				NetworkInterfaceIds: []string{eniID},
			},
		)
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to describe ENI", zap.Error(err))
			continue
		}
		if len(out.NetworkInterfaces) != 1 {
			ts.cfg.Logger.Warn("expected 1 ENI", zap.String("eni", eniID), zap.Int("enis", len(out.NetworkInterfaces)))
			continue
		}
		if out.NetworkInterfaces[0].Attachment == nil {
			ts.cfg.Logger.Warn("no attachment found for ENI", zap.String("eni", eniID))
		} else {
			for i := 0; i < 5; i++ {
				time.Sleep(5 * time.Second)
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.cfg.EC2APIV2.DetachNetworkInterface(
					ctx,
					&aws_ec2_v2.DetachNetworkInterfaceInput{
						AttachmentId: out.NetworkInterfaces[0].Attachment.AttachmentId,
						Force:        aws_v2.Bool(true),
					})
				cancel()
				if err == nil {
					ts.cfg.Logger.Info("successfully detached ENI", zap.String("eni", eniID))
					break
				}
				ts.cfg.Logger.Warn("failed to detach ENI", zap.String("eni", eniID), zap.Error(err))
			}
		}

		for i := 0; i < 5; i++ {
			if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[eniID]; ok {
				break
			}
			//  may take awhile for delete to success upon detach
			time.Sleep(10 * time.Second)
			ts.cfg.Logger.Info("deleting ENI", zap.String("eni", eniID))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.DeleteNetworkInterface(
				ctx,
				&aws_ec2_v2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws_v2.String(eniID),
				})
			cancel()
			if err == nil {
				ts.cfg.Logger.Info("successfully deleted ENI", zap.String("eni", eniID))
				ts.cfg.EKSConfig.Status.DeletedResources[eniID] = "ENI"
				deleted = true
				break
			}
			ts.cfg.Logger.Warn("failed to delete ENI", zap.String("eni", eniID), zap.Error(err))
		}

		// confirm ENI deletion
		retryStart := time.Now()
		for time.Since(retryStart) < 5*time.Minute {
			time.Sleep(5 * time.Second)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.DescribeNetworkInterfaces(
				ctx,
				&aws_ec2_v2.DescribeNetworkInterfacesInput{
					NetworkInterfaceIds: []string{eniID},
				})
			cancel()
			if err == nil {
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, derr := ts.cfg.EC2APIV2.DeleteNetworkInterface(
					ctx,
					&aws_ec2_v2.DeleteNetworkInterfaceInput{
						NetworkInterfaceId: aws_v2.String(eniID),
					})
				cancel()
				ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.Error(derr))
				continue
			}
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if apiErr.ErrorCode() == "InvalidNetworkInterfaceID.NotFound" {
					ts.cfg.Logger.Info("confirmed ENI deletion", zap.String("eni", eniID))
					ts.cfg.EKSConfig.Status.DeletedResources[eniID] = "ENI"
					deleted = true
					break
				}
				ts.cfg.Logger.Warn("failed ENI deletion", zap.String("eni", eniID), zap.String("error-code", apiErr.ErrorCode()))
			}

			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, derr := ts.cfg.EC2APIV2.DeleteNetworkInterface(
				ctx,
				&aws_ec2_v2.DeleteNetworkInterfaceInput{
					NetworkInterfaceId: aws_v2.String(eniID),
				})
			cancel()
			ts.cfg.Logger.Warn("ENI still exists", zap.String("eni", eniID), zap.String("errors", fmt.Sprintf("%v, %v", err, derr)))
		}
	}
	return deleted
}

func (ts *tester) deleteSGs() bool {
	ts.cfg.Logger.Info("deleting security groups for the VPC", zap.String("vpc-id", ts.cfg.EKSConfig.VPC.ID))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	sout, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
		ctx,
		&aws_ec2_v2.DescribeSecurityGroupsInput{
			Filters: []aws_ec2_v2_types.Filter{
				{
					Name:   aws_v2.String("vpc-id"),
					Values: []string{ts.cfg.EKSConfig.VPC.ID},
				},
			},
		})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to describe security groups", zap.Error(err))
		return false
	}

	deleted := false
	for _, sg := range sout.SecurityGroups {
		sgID, sgGroupName := aws_v2.ToString(sg.GroupId), aws_v2.ToString(sg.GroupName)
		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[sgID]; ok {
			continue
		}
		ts.cfg.Logger.Info("cleaning security group",
			zap.String("security-group-id", sgID),
			zap.String("security-group-name", sgGroupName),
		)

		for _, ipPerm := range sg.IpPermissions {
			ts.cfg.Logger.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupIngressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
			} else {
				ts.cfg.Logger.Info("revoked ingress")
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameEgress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgEgress, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDEgress},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgEgress.SecurityGroups) != 1 {
				ts.cfg.Logger.Warn("expected only 1 security group",
					zap.String("egress-security-group-id", sgIDEgress),
					zap.String("egress-security-group-name", sgNameEgress),
					zap.Int("total", len(sgEgress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgEgress.SecurityGroups[0].IpPermissionsEgress {
				ts.cfg.Logger.Info("revoking egress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupEgressInput{
						GroupId:       aws_v2.String(sgIDEgress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				if err != nil {
					ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
				} else {
					ts.cfg.Logger.Info("revoked egress")
					deleted = true
				}
				cancel()
			}
		}

		for _, ipPerm := range sg.IpPermissionsEgress {
			ts.cfg.Logger.Info("revoking egress",
				zap.String("security-group-id", sgID),
				zap.String("ip-perm", fmt.Sprintf("%+v", ipPerm)),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
				ctx,
				&aws_ec2_v2.RevokeSecurityGroupEgressInput{
					GroupId:       aws_v2.String(sgID),
					IpPermissions: []aws_ec2_v2_types.IpPermission{ipPerm},
				})
			cancel()
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if apiErr.ErrorCode() == "InvalidPermission.NotFound" {
						ts.cfg.Logger.Warn("egress ip permission does not exist", zap.Error(err))
						deleted = true
					} else {
						ts.cfg.Logger.Warn("failed to revoke egress", zap.String("error-code", apiErr.ErrorCode()), zap.Error(err))
					}
				} else {
					ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
				}
			} else {
				ts.cfg.Logger.Info("revoked egress",
					zap.String("security-group-id", sgID),
					zap.Error(err),
				)
				deleted = true
			}

			if len(ipPerm.UserIdGroupPairs) != 1 {
				continue
			}
			sgIDIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupId)
			sgNameIngress := aws_v2.ToString(ipPerm.UserIdGroupPairs[0].GroupName)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			sgIngress, err := ts.cfg.EC2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgIDIngress},
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to describe egress security group", zap.Error(err))
				continue
			}
			if len(sgIngress.SecurityGroups) != 1 {
				ts.cfg.Logger.Warn("expected only 1 security group",
					zap.String("ingress-security-group-id", sgIDIngress),
					zap.String("ingress-security-group-name", sgNameIngress),
					zap.Int("total", len(sgIngress.SecurityGroups)),
				)
				continue
			}
			for _, ipPermEg := range sgIngress.SecurityGroups[0].IpPermissionsEgress {
				ts.cfg.Logger.Info("revoking ingress", zap.String("ip-perm", fmt.Sprintf("%+v", ipPermEg)))
				ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
					ctx,
					&aws_ec2_v2.RevokeSecurityGroupIngressInput{
						GroupId:       aws_v2.String(sgIDIngress),
						IpPermissions: []aws_ec2_v2_types.IpPermission{ipPermEg},
					})
				cancel()
				if err != nil {
					var apiErr smithy.APIError
					if errors.As(err, &apiErr) {
						if apiErr.ErrorCode() == "InvalidPermission.NotFound" {
							ts.cfg.Logger.Warn("ingress ip permission does not exist", zap.Error(err))
							deleted = true
						} else {
							ts.cfg.Logger.Warn("failed to revoke ingress", zap.String("error-code", apiErr.ErrorCode()), zap.Error(err))
						}
					} else {
						ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
					}
				} else {
					ts.cfg.Logger.Info("revoked ingress",
						zap.String("security-group-id", sgID),
						zap.Error(err),
					)
					deleted = true
				}
			}
		}

		if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[sgID]; !ok {
			ts.cfg.Logger.Info("deleting security group",
				zap.String("security-group-id", sgID),
				zap.String("security-group-name", sgGroupName),
			)
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.DeleteSecurityGroup(
				ctx,
				&aws_ec2_v2.DeleteSecurityGroupInput{
					GroupId: aws_v2.String(sgID),
				})
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("failed to delete security group", zap.Error(err))
				continue
			}
			ts.cfg.Logger.Info("deleted security group",
				zap.String("security-group-id", sgID),
				zap.String("security-group-name", sgGroupName),
			)
			ts.cfg.EKSConfig.Status.DeletedResources[sgID] = "SecurityGroupID"
			deleted = true
		}

		retryStart := time.Now()
		for time.Since(retryStart) < 5*time.Minute {
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = ts.cfg.EC2APIV2.DescribeSecurityGroups(
				ctx,
				&aws_ec2_v2.DescribeSecurityGroupsInput{
					GroupIds: []string{sgID},
				})
			cancel()
			if err != nil {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), ".NotFound") {
						ts.cfg.Logger.Info("successfully deleted security group",
							zap.String("security-group-id", sgID),
							zap.String("security-group-name", sgGroupName),
						)
						break
					}
					ts.cfg.Logger.Warn("failed to describe securituy group", zap.String("error-code", apiErr.ErrorCode()), zap.Error(err))
				} else {
					ts.cfg.Logger.Warn("failed to describe securituy group", zap.Error(err))
				}
			}
			ts.cfg.Logger.Warn("still deleting security group", zap.Error(err))
			time.Sleep(5 * time.Second)
		}
	}
	return deleted
}
