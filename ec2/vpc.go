package ec2

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

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/cluster/vpc.go for CloudFormation based workflow

func (ts *Tester) createVPC() error {
	fmt.Print(ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.Colorize("[light_green]createVPC [default](%q)\n"), ts.cfg.ConfigPath)

	if ts.cfg.VPC.ID != "" {
		ts.lg.Info("querying ELBv2", zap.String("vpc-id", ts.cfg.VPC.ID))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := ts.elbv2APIV2.DescribeLoadBalancers(
			ctx,
			&aws_elbv2_v2.DescribeLoadBalancersInput{},
		)
		cancel()
		if err != nil {
			ts.lg.Warn("failed to describe ELBv2", zap.Error(err))
		} else {
			for _, ev := range output.LoadBalancers {
				arn := aws_v2.ToString(ev.LoadBalancerArn)
				vpcID := aws_v2.ToString(ev.VpcId)
				if vpcID == ts.cfg.VPC.ID {
					ts.lg.Warn("found ELBv2 for this VPC; may overlap with the other cluster",
						zap.String("vpc-id", ts.cfg.VPC.ID),
						zap.String("elb-arn", arn),
					)
				} else {
					ts.lg.Info("found ELBv2 for other VPCs", zap.String("vpc-id", vpcID), zap.String("elb-arn", arn))
				}
			}
		}

		ts.lg.Info("querying subnet IDs for given VPC",
			zap.String("vpc-id", ts.cfg.VPC.ID),
		)
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		sresp, err := ts.ec2APIV2.DescribeSubnets(
			ctx,
			&aws_ec2_v2.DescribeSubnetsInput{
				Filters: []aws_ec2_v2_types.Filter{
					{
						Name:   aws_v2.String("vpc-id"),
						Values: []string{ts.cfg.VPC.ID},
					},
				},
			})
		cancel()
		if err != nil {
			ts.lg.Warn("failed to subnets", zap.Error(err))
			return err
		}

		ts.cfg.VPC.PublicSubnetIDs = make([]string, 0, len(sresp.Subnets))
		ts.cfg.VPC.PrivateSubnetIDs = make([]string, 0, len(sresp.Subnets))
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
			ts.lg.Info("found subnet",
				zap.String("id", id),
				zap.String("availability-zone", aws_v2.ToString(sv.AvailabilityZone)),
				zap.String("network-tag", networkTagValue),
			)
			switch networkTagValue {
			case "Public":
				ts.cfg.VPC.PublicSubnetIDs = append(ts.cfg.VPC.PublicSubnetIDs, id)
			case "Private":
				ts.cfg.VPC.PrivateSubnetIDs = append(ts.cfg.VPC.PrivateSubnetIDs, id)
			default:
				return fmt.Errorf("'Network' tag not found in subnet %q", id)
			}
		}
		if len(ts.cfg.VPC.PublicSubnetIDs) == 0 {
			return fmt.Errorf("no subnet found for VPC ID %q", ts.cfg.VPC.ID)
		}

		ts.lg.Info("querying security IDs", zap.String("vpc-id", ts.cfg.VPC.ID))
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		gresp, err := ts.ec2APIV2.DescribeSecurityGroups(
			ctx,
			&aws_ec2_v2.DescribeSecurityGroupsInput{
				Filters: []aws_ec2_v2_types.Filter{
					{
						Name:   aws_v2.String("vpc-id"),
						Values: []string{ts.cfg.VPC.ID},
					},
				},
			})
		cancel()
		if err != nil {
			ts.lg.Warn("failed to security groups", zap.Error(err))
			return err
		}
		for _, sg := range gresp.SecurityGroups {
			id, name := aws_v2.ToString(sg.GroupId), aws_v2.ToString(sg.GroupName)
			ts.lg.Info("found security group", zap.String("id", id), zap.String("name", name))
			if name != "default" {
				ts.cfg.VPC.SecurityGroupID = id
			}
		}
		if ts.cfg.VPC.SecurityGroupID == "" {
			return fmt.Errorf("no security group found for VPC ID %q", ts.cfg.VPC.ID)
		}

		ts.cfg.Sync()
		return nil
	}
	if !ts.cfg.VPC.Create {
		ts.lg.Info("VPC.Create false; skipping creation")
		return nil
	}
	if ts.cfg.VPC.ID != "" &&
		len(ts.cfg.VPC.PublicSubnetIDs) > 0 &&
		ts.cfg.VPC.SecurityGroupID != "" {
		ts.lg.Info("VPC already created; no need to create a new one")
		return nil
	}

	if err := ts._createVPC(); err != nil { // AWS::EC2::VPC
		return err
	}
	if err := ts.modifyVPC(); err != nil {
		return err
	}
	if err := ts.createSecurityGroups(); err != nil { // AWS::EC2::SecurityGroup
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

	if err := ts.authorizeSecurityGroup(); err != nil {
		return err
	}

	ts.lg.Info("created a VPC",
		zap.String("vpc-id", ts.cfg.VPC.ID),
		zap.Strings("vpc-cidr-blocks", ts.cfg.VPC.CIDRs),
		zap.Strings("public-subnet-ids", ts.cfg.VPC.PublicSubnetIDs),
		zap.Strings("private-subnet-ids", ts.cfg.VPC.PrivateSubnetIDs),
		zap.String("control-plane-security-group-id", ts.cfg.VPC.SecurityGroupID),
	)

	ts.cfg.Sync()
	return nil
}

// e.g. DependencyViolation: The vpc 'vpc-0127f6d18bd98836a' has dependencies and cannot be deleted
func (ts *Tester) deleteVPC() error {
	fmt.Print(ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.Colorize("[light_blue]deleteVPC [default](%q)\n"), ts.cfg.ConfigPath)

	if !ts.cfg.VPC.Create {
		ts.lg.Info("VPC.Create false; skipping deletion")
		return nil
	}

	waitDur := 30 * time.Second
	ts.lg.Info("sleeping before VPC deletion", zap.Duration("wait", waitDur))
	time.Sleep(waitDur)

	ts.lg.Info("deleting VPC", zap.String("vpc-id", ts.cfg.VPC.ID))

	var errs []string

	if err := ts.revokeSecurityGroups(); err != nil {
		ts.lg.Warn("failed to delete ELB v2", zap.Error(err))
		time.Sleep(10 * time.Second) // retry
	}
	if err := ts.deleteELBv2(); err != nil {
		ts.lg.Warn("failed to delete ELB v2", zap.Error(err))
		time.Sleep(10 * time.Second) // retry
	}
	if err := ts.deleteDHCPOptions(); err != nil {
		ts.lg.Warn("failed to delete DHCP options", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if err := ts.deletePrivateSubnetRouteTableAssociation(); err != nil {
		ts.lg.Warn("failed to delete subnet route table association", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePrivateRouteTables(); err != nil {
		ts.lg.Warn("failed to delete private route tables", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePrivateSubnets(); err != nil {
		ts.lg.Warn("failed to delete private subnets", zap.Error(err))
		time.Sleep(10 * time.Second)
	}

	select {
	case <-time.After(30 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deleteELBv2(); err != nil {
		ts.lg.Warn("failed to delete ELB v2", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.revokeSecurityGroups(); err != nil {
		ts.lg.Warn("failed to revoke security groups", zap.Error(err))
		errs = append(errs, err.Error())
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deletePublicNATGateways(); err != nil {
		ts.lg.Warn("failed to delete public NAT gateways", zap.Error(err))
		time.Sleep(20 * time.Second)
	}
	if err := ts.deletePublicEIPs(); err != nil {
		ts.lg.Warn("failed to delete public EIPs", zap.Error(err))
		time.Sleep(10 * time.Second)
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	// try again, NAT gateway delete may take some time
	if err := ts.deletePublicNATGateways(); err != nil {
		ts.lg.Warn("failed to delete public NAT gateways", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePublicEIPs(); err != nil {
		ts.lg.Warn("failed to delete public EIPs", zap.Error(err))
		errs = append(errs, err.Error())
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deletePublicSubnetRouteTableAssociation(); err != nil {
		ts.lg.Warn("failed to delete public subnet route table association", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePublicRouteTable(); err != nil {
		ts.lg.Warn("failed to delete public route table", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePrivateSubnets(); err != nil {
		ts.lg.Warn("failed to delete private subnets", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePublicSubnets(); err != nil {
		ts.lg.Warn("failed to delete public subnets", zap.Error(err))
		errs = append(errs, err.Error())
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deleteVPCGatewayAttachment(); err != nil {
		ts.lg.Warn("failed to VPC gateway attachment", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteInternetGateway(); err != nil {
		ts.lg.Warn("failed to delete internet gateway", zap.Error(err))
		errs = append(errs, err.Error())
	}

	select {
	case <-time.After(10 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deleteOtherSecurityGroups(); err != nil {
		ts.lg.Warn("failed to delete sg", zap.Error(err))
		time.Sleep(10 * time.Second)
	}
	if err := ts.deleteSecurityGroups(); err != nil {
		ts.lg.Warn("failed to delete ENIs", zap.Error(err))
		time.Sleep(10 * time.Second)
	}

	select {
	case <-time.After(20 * time.Second):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	if err := ts.deleteSecurityGroups(); err != nil {
		ts.lg.Warn("failed to delete ENIs", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if err := ts._deleteVPC(); err != nil {
		ts.lg.Warn("failed to delete VPC", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ","))
	}
	return nil
}

// AWS::EC2::VPC
func (ts *Tester) _createVPC() error {
	ts.lg.Info("creating a VPC", zap.String("cidr-block", ts.cfg.VPC.CIDRs[0]))
	vpcOut, err := ts.ec2APIV2.CreateVpc(
		context.Background(),
		&aws_ec2_v2.CreateVpcInput{
			CidrBlock: aws_v2.String(ts.cfg.VPC.CIDRs[0]),
			TagSpecifications: []aws_ec2_v2_types.TagSpecification{
				{
					ResourceType: aws_ec2_v2_types.ResourceTypeVpc,
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String("Name"),
							Value: aws_v2.String(fmt.Sprintf("%s-vpc", ts.cfg.Name)),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.lg.Warn("failed to create a VPC", zap.Error(err))
		return err
	}

	ts.cfg.VPC.ID = aws_v2.ToString(vpcOut.Vpc.VpcId)
	ts.cfg.Sync()
	ts.lg.Info("created a VPC", zap.String("vpc-id", ts.cfg.VPC.ID))
	return nil
}

func (ts *Tester) modifyVPC() error {
	ts.lg.Info("modifying VPC attributes", zap.String("vpc-id", ts.cfg.VPC.ID))
	_, err := ts.ec2APIV2.ModifyVpcAttribute(
		context.Background(),
		&aws_ec2_v2.ModifyVpcAttributeInput{
			VpcId:              aws_v2.String(ts.cfg.VPC.ID),
			EnableDnsHostnames: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
		},
	)
	if err != nil {
		ts.lg.Warn("failed to modify VPC attribute EnableDnsHostnames", zap.Error(err))
		return err
	}
	_, err = ts.ec2APIV2.ModifyVpcAttribute(
		context.Background(),
		&aws_ec2_v2.ModifyVpcAttributeInput{
			VpcId:            aws_v2.String(ts.cfg.VPC.ID),
			EnableDnsSupport: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
		},
	)
	if err != nil {
		ts.lg.Warn("failed to modify VPC attribute EnableDnsSupport", zap.Error(err))
		return err
	}

	ts.lg.Info("modifid VPC attributes")
	return nil
}

func (ts *Tester) _deleteVPC() (err error) {
	ts.lg.Info("deleting VPC")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.ID]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DeleteVpc(
		context.Background(),
		&aws_ec2_v2.DeleteVpcInput{
			VpcId: aws_v2.String(ts.cfg.VPC.ID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to detach VPC", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.ID] = "VPC.ID"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.lg.Info("deleted VPC")
	ts.cfg.DeletedResources[ts.cfg.VPC.ID] = "VPC.ID"
	ts.cfg.Sync()

	return nil
}

// AWS::EC2::VPCCidrBlock
func (ts *Tester) associateVPCCIDRBlocks() error {
	ts.lg.Info("associating VPC CIDR blocks with the rest")
	for _, cidr := range ts.cfg.VPC.CIDRs[1:] {
		_, err := ts.ec2APIV2.AssociateVpcCidrBlock(
			context.Background(),
			&aws_ec2_v2.AssociateVpcCidrBlockInput{
				VpcId:     aws_v2.String(ts.cfg.VPC.ID),
				CidrBlock: aws_v2.String(cidr),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to associate VPC CIDR block", zap.String("cidr-block", cidr), zap.Error(err))
			return err
		}
		ts.lg.Info("associated VPC CIDR block", zap.String("cidr-block", cidr))
	}
	return nil
}

// AWS::EC2::InternetGateway
func (ts *Tester) createInternetGateway() error {
	ts.lg.Info("creating internet gateway")
	out, err := ts.ec2APIV2.CreateInternetGateway(
		context.Background(),
		&aws_ec2_v2.CreateInternetGatewayInput{},
	)
	if err != nil {
		ts.lg.Warn("failed to create internet gateway", zap.Error(err))
		return err
	}

	ts.cfg.VPC.InternetGatewayID = aws_v2.ToString(out.InternetGateway.InternetGatewayId)
	ts.cfg.Sync()
	ts.lg.Info("created internet gateway", zap.String("internet-gateway-id", ts.cfg.VPC.InternetGatewayID))

	return nil
}

func (ts *Tester) deleteInternetGateway() (err error) {
	ts.lg.Info("deleting internet gateway")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DeleteInternetGateway(
		context.Background(),
		&aws_ec2_v2.DeleteInternetGatewayInput{
			InternetGatewayId: aws_v2.String(ts.cfg.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete internet gateway", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID] = "VPC.InternetGatewayID"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID] = "VPC.InternetGatewayID"
	ts.cfg.Sync()
	ts.lg.Info("deleted internet gateway")

	return nil
}

// AWS::EC2::VPCGatewayAttachment
func (ts *Tester) createVPCGatewayAttachment() error {
	ts.lg.Info("creating VPC gateway attachment")
	_, err := ts.ec2APIV2.AttachInternetGateway(
		context.Background(),
		&aws_ec2_v2.AttachInternetGatewayInput{
			VpcId:             aws_v2.String(ts.cfg.VPC.ID),
			InternetGatewayId: aws_v2.String(ts.cfg.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to create VPC gateway attachment", zap.Error(err))
		return err
	}

	ts.lg.Info("created VPC gateway attachment")
	return nil
}

func (ts *Tester) deleteVPCGatewayAttachment() (err error) {
	ts.lg.Info("deleting VPC gateway attachment")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID+"-detach"]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DetachInternetGateway(
		context.Background(),
		&aws_ec2_v2.DetachInternetGatewayInput{
			VpcId:             aws_v2.String(ts.cfg.VPC.ID),
			InternetGatewayId: aws_v2.String(ts.cfg.VPC.InternetGatewayID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to detach VPC gateway attachment", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID+"-detach"] = "VPC.InternetGatewayID.detach"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.DeletedResources[ts.cfg.VPC.InternetGatewayID+"-detach"] = "VPC.InternetGatewayID.detach"
	ts.cfg.Sync()
	ts.lg.Info("deleted VPC gateway attachment")

	return nil
}

// AWS::EC2::Subnet
func (ts *Tester) createPublicSubnets() error {
	ts.lg.Info("creating public subnets", zap.Strings("availability-zones", ts.cfg.AvailabilityZoneNames))

	cidrs := make([]string, len(ts.cfg.AvailabilityZoneNames))
	copy(cidrs, ts.cfg.VPC.PublicSubnetCIDRs)

	ts.cfg.VPC.PublicSubnetIDs = make([]string, 0)
	for idx, cidr := range cidrs {
		sout, err := ts.ec2APIV2.CreateSubnet(
			context.Background(),
			&aws_ec2_v2.CreateSubnetInput{
				VpcId:            aws_v2.String(ts.cfg.VPC.ID),
				AvailabilityZone: aws_v2.String(ts.cfg.AvailabilityZoneNames[idx]),
				CidrBlock:        aws_v2.String(cidr),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						ResourceType: aws_ec2_v2_types.ResourceTypeSubnet,
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-public-subnet-%d", ts.cfg.Name, idx+1)),
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
			ts.lg.Warn("failed to create public subnets", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		subnetID := aws_v2.ToString(sout.Subnet.SubnetId)
		ts.cfg.VPC.PublicSubnetIDs = append(ts.cfg.VPC.PublicSubnetIDs, subnetID)
		ts.lg.Info("created a public subnet", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))

		_, err = ts.ec2APIV2.ModifySubnetAttribute(
			context.Background(),
			&aws_ec2_v2.ModifySubnetAttributeInput{
				SubnetId:            sout.Subnet.SubnetId,
				MapPublicIpOnLaunch: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(true)},
			},
		)
		if err != nil {
			ts.lg.Warn("failed to modify subnet attribute", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		ts.lg.Info("modified the public subnet with MapPublicIpOnLaunch", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))
	}
	ts.cfg.Sync()
	ts.lg.Info("created public subnets", zap.Strings("availability-zones", ts.cfg.AvailabilityZoneNames))

	return nil
}

func (ts *Tester) deletePublicSubnets() (err error) {
	ts.lg.Info("deleting public subnets")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, subnet := range ts.cfg.VPC.PublicSubnetIDs {
		if _, ok := ts.cfg.DeletedResources[subnet]; ok {
			continue
		}
		_, err := ts.ec2APIV2.DeleteSubnet(
			context.Background(),
			&aws_ec2_v2.DeleteSubnetInput{
				SubnetId: aws_v2.String(subnet),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to delete public subnet", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[subnet] = "VPC.PublicSubnetID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[subnet] = "VPC.PublicSubnetID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted public subnets")
		return nil
	}
	return errors.New("failed to delete all public subnets")
}

// AWS::EC2::RouteTable
func (ts *Tester) createPublicRouteTable() error {
	ts.lg.Info("creating public route table")

	out, err := ts.ec2APIV2.CreateRouteTable(
		context.Background(),
		&aws_ec2_v2.CreateRouteTableInput{
			VpcId: aws_v2.String(ts.cfg.VPC.ID),
			TagSpecifications: []aws_ec2_v2_types.TagSpecification{
				{
					ResourceType: aws_ec2_v2_types.ResourceTypeRouteTable,
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String("Name"),
							Value: aws_v2.String(fmt.Sprintf("%s-public-route-table", ts.cfg.Name)),
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
		ts.lg.Warn("failed to create public route table", zap.Error(err))
		return err
	}

	ts.cfg.VPC.PublicRouteTableID = aws_v2.ToString(out.RouteTable.RouteTableId)
	ts.cfg.Sync()

	ts.lg.Info("created public route table", zap.String("route-table-id", ts.cfg.VPC.PublicRouteTableID))
	return nil
}

func (ts *Tester) deletePublicRouteTable() (err error) {
	ts.lg.Info("deleting public route table")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if ts.cfg.VPC.PublicRouteTableID == "" {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.PublicRouteTableID]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DeleteRouteTable(
		context.Background(),
		&aws_ec2_v2.DeleteRouteTableInput{
			RouteTableId: aws_v2.String(ts.cfg.VPC.PublicRouteTableID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete public route table", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.PublicRouteTableID] = "VPC.PublicRouteTableID"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.DeletedResources[ts.cfg.VPC.PublicRouteTableID] = "VPC.PublicRouteTableID"
	ts.cfg.Sync()

	ts.lg.Info("deleted public route table")
	return nil
}

// AWS::EC2::Route
func (ts *Tester) createPublicRoute() error {
	ts.lg.Info("creating public route")

	_, err := ts.ec2APIV2.CreateRoute(
		context.Background(),
		&aws_ec2_v2.CreateRouteInput{
			RouteTableId:         aws_v2.String(ts.cfg.VPC.PublicRouteTableID),
			GatewayId:            aws_v2.String(ts.cfg.VPC.InternetGatewayID),
			DestinationCidrBlock: aws_v2.String("0.0.0.0/0"),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to create public route table", zap.Error(err))
		return err
	}

	ts.lg.Info("created public route")
	return nil
}

// AWS::EC2::SubnetRouteTableAssociation
func (ts *Tester) createPublicSubnetRouteTableAssociation() error {
	ts.lg.Info("creating public subnet route table association")

	for _, subnet := range ts.cfg.VPC.PublicSubnetIDs {
		out, err := ts.ec2APIV2.AssociateRouteTable(
			context.Background(),
			&aws_ec2_v2.AssociateRouteTableInput{
				SubnetId:     aws_v2.String(subnet),
				RouteTableId: aws_v2.String(ts.cfg.VPC.PublicRouteTableID),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to associate route table", zap.Error(err))
			return err
		}
		ts.cfg.VPC.PublicSubnetRouteTableAssociationIDs = append(ts.cfg.VPC.PublicSubnetRouteTableAssociationIDs, aws_v2.ToString(out.AssociationId))
	}
	ts.cfg.Sync()

	ts.lg.Info("created public subnet route table association")
	return nil
}

func (ts *Tester) deletePublicSubnetRouteTableAssociation() (err error) {
	ts.lg.Info("deleting public subnet route table association")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, id := range ts.cfg.VPC.PublicSubnetRouteTableAssociationIDs {
		if _, ok := ts.cfg.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.ec2APIV2.DisassociateRouteTable(
			context.Background(),
			&aws_ec2_v2.DisassociateRouteTableInput{
				AssociationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to disassociate route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[id] = "VPC.PublicSubnetRouteTableAssociationID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[id] = "VPC.PublicSubnetRouteTableAssociationID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted public subnet route table association")
		return nil
	}
	return errors.New("failed to delete all public subnet route table association")
}

// AWS::EC2::EIP
func (ts *Tester) createPublicEIPs() error {
	ts.lg.Info("creating public EIPs")

	for idx := range ts.cfg.VPC.PublicSubnetIDs {
		// tags are not supported in ISO regions
		tags := make([]aws_ec2_v2_types.TagSpecification, 0)
		if !strings.Contains(ts.cfg.Partition, "-iso") {
			tags = []aws_ec2_v2_types.TagSpecification{
				{
					ResourceType: aws_ec2_v2_types.ResourceTypeElasticIp,
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String("Name"),
							Value: aws_v2.String(fmt.Sprintf("%s-eip-%d", ts.cfg.Name, idx+1)),
						},
					},
				},
			}
		}
		out, err := ts.ec2APIV2.AllocateAddress(
			context.Background(),
			&aws_ec2_v2.AllocateAddressInput{
				Domain:            aws_ec2_v2_types.DomainTypeVpc,
				TagSpecifications: tags,
			},
		)
		if err != nil {
			ts.lg.Warn("failed to create EIP", zap.Error(err))
			return err
		}
		ts.lg.Info("created EIP",
			zap.String("public-ip", aws_v2.ToString(out.PublicIp)),
			zap.String("allocation-id", aws_v2.ToString(out.AllocationId)),
		)
		ts.cfg.VPC.EIPAllocationIDs = append(ts.cfg.VPC.EIPAllocationIDs, aws_v2.ToString(out.AllocationId))
	}
	ts.cfg.Sync()

	ts.lg.Info("created public EIPs")
	return nil
}

func (ts *Tester) deletePublicEIPs() (err error) {
	ts.lg.Info("deleting public EIPs")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, id := range ts.cfg.VPC.EIPAllocationIDs {
		if _, ok := ts.cfg.DeletedResources[id]; ok {
			continue
		}
		_, err := ts.ec2APIV2.ReleaseAddress(
			context.Background(),
			&aws_ec2_v2.ReleaseAddressInput{
				AllocationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to delete EIP", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[id] = "VPC.EIPAllocationID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[id] = "VPC.EIPAllocationID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted public EIPs")
		return nil
	}
	return errors.New("failed to delete all public EIPs")
}

// AWS::EC2::NatGateway
func (ts *Tester) createPublicNATGateways() error {
	ts.lg.Info("creating public NAT gateways using EIPs and public subnets")

	for idx, subnet := range ts.cfg.VPC.PublicSubnetIDs {
		out, err := ts.ec2APIV2.CreateNatGateway(
			context.Background(),
			&aws_ec2_v2.CreateNatGatewayInput{
				SubnetId:     aws_v2.String(subnet),
				AllocationId: aws_v2.String(ts.cfg.VPC.EIPAllocationIDs[idx]),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						ResourceType: aws_ec2_v2_types.ResourceTypeNatgateway,
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-nat-gateway-%d", ts.cfg.Name, idx+1)),
							},
						},
					},
				},
			},
		)
		if err != nil {
			ts.lg.Warn("failed to create NAT gateway", zap.Error(err))
			return err
		}
		ts.lg.Info("created NAT gateway", zap.String("nat-gateway-id", aws_v2.ToString(out.NatGateway.NatGatewayId)))
		ts.cfg.VPC.NATGatewayIDs = append(ts.cfg.VPC.NATGatewayIDs, aws_v2.ToString(out.NatGateway.NatGatewayId))
	}
	ts.cfg.Sync()

	select {
	case <-time.After(time.Minute):
	case <-ts.stopCreationCh:
		return errors.New("stopped")
	}

	// make sure NAT gateway creation is complete
	// otherwise, it will fail
	// e.g.,
	// "operation error EC2: CreateRoute, https response error StatusCode: 400, RequestID: 3ad484f1-0349-4002-89df-043eeed8fe0c, api error InvalidGatewayID.NotFound: The gateway ID 'nat-0915fd7674db44e54' does not exist"
	for _, id := range ts.cfg.VPC.NATGatewayIDs {
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Second)
			out, err := ts.ec2APIV2.DescribeNatGateways(
				context.Background(),
				&aws_ec2_v2.DescribeNatGatewaysInput{
					NatGatewayIds: []string{id},
				},
			)
			if err != nil {
				ts.lg.Warn("failed to describe NAT gateway", zap.String("nat-gateway-id", id), zap.Error(err))
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					if strings.Contains(apiErr.ErrorCode(), "NotFound") {
						time.Sleep(5 * time.Second)
					}
				}
				continue
			}
			if len(out.NatGateways) != 1 {
				ts.lg.Warn("expected 1 NAT gateway", zap.Int("nat-gateways", len(out.NatGateways)))
				continue
			}
			nat := out.NatGateways[0]
			ts.lg.Info("described NAT gateway",
				zap.String("id", aws_v2.ToString(nat.NatGatewayId)),
				zap.String("state", fmt.Sprint(nat.State)),
				zap.String("subnet-id", aws_v2.ToString(nat.SubnetId)),
				zap.String("connectivity-type", fmt.Sprint(nat.ConnectivityType)),
				zap.String("failure-message", aws_v2.ToString(nat.FailureMessage)),
			)
			// TODO: if pending, retry
			if nat.State != aws_ec2_v2_types.NatGatewayStateAvailable {
				continue
			}
			break
		}
	}

	ts.lg.Info("created public NAT gateways")
	return nil
}

func (ts *Tester) deletePublicNATGateways() (err error) {
	ts.lg.Info("deleting public NAT gateways")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, id := range ts.cfg.VPC.NATGatewayIDs {
		if _, ok := ts.cfg.DeletedResources[id]; ok {
			continue
		}

		_, err := ts.ec2APIV2.DeleteNatGateway(
			context.Background(),
			&aws_ec2_v2.DeleteNatGatewayInput{
				NatGatewayId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to delete public NAT gateway", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[id] = "VPC.NATGatewayID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Second)
			_, err1 := ts.ec2APIV2.DeleteNatGateway(
				context.Background(),
				&aws_ec2_v2.DeleteNatGatewayInput{
					NatGatewayId: aws_v2.String(id),
				},
			)
			_, err2 := ts.ec2APIV2.DescribeNatGateways(
				context.Background(),
				&aws_ec2_v2.DescribeNatGatewaysInput{
					NatGatewayIds: []string{id},
				},
			)
			if err2 == nil {
				continue
			}
			ts.lg.Warn("failed to describe NAT gateway during deletion",
				zap.String("delete-error", fmt.Sprintf("%v", err1)),
				zap.Error(err2),
			)
			var apiErr smithy.APIError
			if errors.As(err2, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[id] = "VPC.NATGatewayID"
					ts.cfg.Sync()
					break
				}
			}
		}
		ts.cfg.DeletedResources[id] = "VPC.NATGatewayID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted public NAT gateways")
		return nil
	}
	return errors.New("failed to delete all public NAT gateways")
}

// AWS::EC2::Subnet
func (ts *Tester) createPrivateSubnets() error {
	ts.lg.Info("creating private subnets", zap.Strings("availability-zones", ts.cfg.AvailabilityZoneNames))

	ts.cfg.VPC.PrivateSubnetIDs = make([]string, 0)
	for idx, cidr := range ts.cfg.VPC.PrivateSubnetCIDRs {
		sout, err := ts.ec2APIV2.CreateSubnet(
			context.Background(),
			&aws_ec2_v2.CreateSubnetInput{
				VpcId:            aws_v2.String(ts.cfg.VPC.ID),
				AvailabilityZone: aws_v2.String(ts.cfg.AvailabilityZoneNames[idx]),
				CidrBlock:        aws_v2.String(cidr),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						ResourceType: aws_ec2_v2_types.ResourceTypeSubnet,
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-private-subnet-%d", ts.cfg.Name, idx+1)),
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
			ts.lg.Warn("failed to create private subnets", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		subnetID := aws_v2.ToString(sout.Subnet.SubnetId)
		ts.cfg.VPC.PrivateSubnetIDs = append(ts.cfg.VPC.PrivateSubnetIDs, subnetID)
		ts.lg.Info("created a private subnet", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))

		_, err = ts.ec2APIV2.ModifySubnetAttribute(
			context.Background(),
			&aws_ec2_v2.ModifySubnetAttributeInput{
				SubnetId:            sout.Subnet.SubnetId,
				MapPublicIpOnLaunch: &aws_ec2_v2_types.AttributeBooleanValue{Value: aws_v2.Bool(false)},
			},
		)
		if err != nil {
			ts.lg.Warn("failed to modify subnet attribute", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.Error(err))
			return err
		}
		ts.lg.Info("modified the private subnet with MapPublicIpOnLaunch", zap.String("availability-zone", ts.cfg.AvailabilityZoneNames[idx]), zap.String("subnet-id", subnetID))
	}
	ts.cfg.Sync()

	ts.lg.Info("created private subnets", zap.Strings("availability-zones", ts.cfg.AvailabilityZoneNames))
	return nil
}

func (ts *Tester) deletePrivateSubnets() (err error) {
	ts.lg.Info("deleting private subnets")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, subnet := range ts.cfg.VPC.PrivateSubnetIDs {
		if _, ok := ts.cfg.DeletedResources[subnet]; ok {
			continue
		}
		_, err := ts.ec2APIV2.DeleteSubnet(
			context.Background(),
			&aws_ec2_v2.DeleteSubnetInput{
				SubnetId: aws_v2.String(subnet),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to delete private subnet", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[subnet] = "VPC.PrivateSubnetID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[subnet] = "VPC.PrivateSubnetID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted private subnets")
		return nil
	}
	return errors.New("failed to delete all private subnets")
}

// AWS::EC2::RouteTable
func (ts *Tester) createPrivateRouteTables() error {
	ts.lg.Info("creating private route tables using VPC")

	for idx := range ts.cfg.VPC.PrivateSubnetIDs {
		out, err := ts.ec2APIV2.CreateRouteTable(
			context.Background(),
			&aws_ec2_v2.CreateRouteTableInput{
				VpcId: aws_v2.String(ts.cfg.VPC.ID),
				TagSpecifications: []aws_ec2_v2_types.TagSpecification{
					{
						ResourceType: aws_ec2_v2_types.ResourceTypeRouteTable,
						Tags: []aws_ec2_v2_types.Tag{
							{
								Key:   aws_v2.String("Name"),
								Value: aws_v2.String(fmt.Sprintf("%s-private-route-table-%d", ts.cfg.Name, idx+1)),
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
			ts.lg.Warn("failed to create private route table", zap.Error(err))
			return err
		}
		ts.cfg.VPC.PrivateRouteTableIDs = append(ts.cfg.VPC.PrivateRouteTableIDs, aws_v2.ToString(out.RouteTable.RouteTableId))
	}
	ts.cfg.Sync()

	ts.lg.Info("created private route tables", zap.Strings("route-table-ids", ts.cfg.VPC.PrivateRouteTableIDs))
	return nil
}

func (ts *Tester) deletePrivateRouteTables() (err error) {
	ts.lg.Info("deleting private route tables")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, routeTableID := range ts.cfg.VPC.PrivateRouteTableIDs {
		if _, ok := ts.cfg.DeletedResources[routeTableID]; ok {
			continue
		}
		_, err := ts.ec2APIV2.DeleteRouteTable(
			context.Background(),
			&aws_ec2_v2.DeleteRouteTableInput{
				RouteTableId: aws_v2.String(routeTableID),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to delete private route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[routeTableID] = "VPC.PrivateRouteTableID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[routeTableID] = "VPC.PrivateRouteTableID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted private route tables")
		return nil
	}
	return errors.New("failed to delete all private routes")
}

// AWS::EC2::Route
func (ts *Tester) createPrivateRoutes() error {
	ts.lg.Info("creating private routes using NAT gateways and private route tables")

	// we create 3 public subnets and 3 NAT gateways for 3-AZ regions
	// the last NAT gateway may be unmapped
	for idx, route := range ts.cfg.VPC.PrivateRouteTableIDs {
		_, err := ts.ec2APIV2.CreateRoute(
			context.Background(),
			&aws_ec2_v2.CreateRouteInput{
				RouteTableId:         aws_v2.String(route),
				GatewayId:            aws_v2.String(ts.cfg.VPC.NATGatewayIDs[idx]),
				DestinationCidrBlock: aws_v2.String("0.0.0.0/0"),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to create private route table", zap.Error(err))
			return err
		}
	}

	ts.lg.Info("created private routes")
	return nil
}

// AWS::EC2::SubnetRouteTableAssociation
func (ts *Tester) createPrivateSubnetRouteTableAssociation() error {
	ts.lg.Info("creating private subnet route table association using private route table")

	for idx, subnet := range ts.cfg.VPC.PrivateSubnetIDs {
		out, err := ts.ec2APIV2.AssociateRouteTable(
			context.Background(),
			&aws_ec2_v2.AssociateRouteTableInput{
				SubnetId:     aws_v2.String(subnet),
				RouteTableId: aws_v2.String(ts.cfg.VPC.PrivateRouteTableIDs[idx]),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to associate route table", zap.Error(err))
			return err
		}
		ts.cfg.VPC.PrivateSubnetRouteTableAssociationIDs = append(ts.cfg.VPC.PrivateSubnetRouteTableAssociationIDs, aws_v2.ToString(out.AssociationId))
	}
	ts.cfg.Sync()

	ts.lg.Info("created private subnet route table association")
	return nil
}

func (ts *Tester) deletePrivateSubnetRouteTableAssociation() (err error) {
	ts.lg.Info("deleting private subnet route table association")
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	deleted := true
	for _, id := range ts.cfg.VPC.PrivateSubnetRouteTableAssociationIDs {
		if _, ok := ts.cfg.DeletedResources[id]; ok {
			continue
		}
		_, err = ts.ec2APIV2.DisassociateRouteTable(
			context.Background(),
			&aws_ec2_v2.DisassociateRouteTableInput{
				AssociationId: aws_v2.String(id),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to disassociate route table", zap.Error(err))
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[id] = "VPC.PrivateSubnetRouteTableAssociationID"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}
		ts.cfg.DeletedResources[id] = "VPC.PrivateSubnetRouteTableAssociationID"
		ts.cfg.Sync()
	}

	if deleted {
		ts.lg.Info("deleted private subnet route table association")
		return nil
	}
	return errors.New("failed to delete all private subnet route table association")
}

// AWS::EC2::DHCPOptions
// AWS::EC2::VPCDHCPOptionsAssociation
func (ts *Tester) createDHCPOptions() error {
	ts.lg.Info("creating DHCP options domain name and servers")
	if ts.cfg.VPC.ID == "" {
		return errors.New("empty VPC.ID")
	}
	if len(ts.cfg.VPC.DHCPOptionsDomainName) == 0 && len(ts.cfg.VPC.DHCPOptionsDomainNameServers) == 0 {
		return nil
	}

	dhcpConfigs := make([]aws_ec2_v2_types.NewDhcpConfiguration, 0)
	if ts.cfg.VPC.DHCPOptionsDomainName != "" {
		dhcpConfigs = append(dhcpConfigs, aws_ec2_v2_types.NewDhcpConfiguration{
			Key:    aws_v2.String("DomainName"),
			Values: []string{ts.cfg.VPC.DHCPOptionsDomainName},
		})
	}
	if len(ts.cfg.VPC.DHCPOptionsDomainNameServers) > 0 {
		dhcpConfigs = append(dhcpConfigs, aws_ec2_v2_types.NewDhcpConfiguration{
			Key:    aws_v2.String("DomainNameServers"),
			Values: ts.cfg.VPC.DHCPOptionsDomainNameServers,
		})
	}

	dhcpOut, err := ts.ec2APIV2.CreateDhcpOptions(
		context.Background(),
		&aws_ec2_v2.CreateDhcpOptionsInput{
			DhcpConfigurations: dhcpConfigs,
		},
	)
	if err != nil {
		ts.lg.Warn("failed to associate DHCP options", zap.Error(err))
		return err
	}

	ts.cfg.VPC.DHCPOptionsID = aws_v2.ToString(dhcpOut.DhcpOptions.DhcpOptionsId)
	ts.cfg.Sync()

	_, err = ts.ec2APIV2.AssociateDhcpOptions(
		context.Background(),
		&aws_ec2_v2.AssociateDhcpOptionsInput{
			DhcpOptionsId: dhcpOut.DhcpOptions.DhcpOptionsId,
			VpcId:         aws_v2.String(ts.cfg.VPC.ID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to associate DHCP options", zap.Error(err))
		return err
	}

	ts.lg.Info("created and associated DHCP options domain name and servers")
	return nil
}

func (ts *Tester) deleteDHCPOptions() (err error) {
	ts.lg.Info("deleting DHCP options domain name and servers")
	if ts.cfg.VPC.ID == "" {
		return nil
	}
	if len(ts.cfg.VPC.DHCPOptionsDomainName) == 0 && len(ts.cfg.VPC.DHCPOptionsDomainNameServers) == 0 {
		return nil
	}
	if _, ok := ts.cfg.DeletedResources[ts.cfg.VPC.DHCPOptionsID]; ok {
		return nil
	}

	_, err = ts.ec2APIV2.DeleteDhcpOptions(
		context.Background(),
		&aws_ec2_v2.DeleteDhcpOptionsInput{
			DhcpOptionsId: aws_v2.String(ts.cfg.VPC.DHCPOptionsID),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete DHCP options", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.VPC.DHCPOptionsID] = "VPC.DHCPOptionsID"
				ts.cfg.Sync()
				return nil
			}
		}
		return nil
	}

	ts.cfg.DeletedResources[ts.cfg.VPC.DHCPOptionsID] = "VPC.DHCPOptionsID"
	ts.cfg.Sync()

	ts.lg.Info("deleted and disassociated DHCP options domain name and servers")
	return nil
}

func (ts *Tester) deleteELBv2() (err error) {
	ts.lg.Info("deleting ELBv2 for the VPC", zap.String("vpc-id", ts.cfg.VPC.ID))
	if ts.cfg.VPC.ID == "" {
		return nil
	}

	output, err := ts.elbv2APIV2.DescribeLoadBalancers(
		context.Background(),
		&aws_elbv2_v2.DescribeLoadBalancersInput{},
	)
	if err != nil {
		ts.lg.Warn("failed to describe ELBv2", zap.Error(err))
		return err
	}
	if len(output.LoadBalancers) == 0 {
		ts.lg.Info("ELBv2 not found")
		return nil
	}

	elbARNs := make([]string, 0)
	for _, ev := range output.LoadBalancers {
		vpcID := aws_v2.ToString(ev.VpcId)
		elbV2ARN := aws_v2.ToString(ev.LoadBalancerArn)
		if _, ok := ts.cfg.DeletedResources[elbV2ARN]; ok {
			continue
		}
		if vpcID != ts.cfg.VPC.ID {
			ts.lg.Info("found ELBv2 for other VPCs",
				zap.String("vpc-id", vpcID),
				zap.String("elb-v2-arn", elbV2ARN),
			)
			continue
		}
		elbARNs = append(elbARNs, elbV2ARN)
		ts.lg.Info("found ELBv2 for this VPC",
			zap.String("vpc-id", vpcID),
			zap.String("elb-v2-arn", elbV2ARN),
		)
	}

	deleted := true
	for _, arn := range elbARNs {
		ts.lg.Info("removing ELBv2",
			zap.String("vpc-id", ts.cfg.VPC.ID),
			zap.String("elb-v2-arn", arn),
		)

		_, err = ts.elbv2APIV2.DeleteLoadBalancer(
			context.Background(),
			&aws_elbv2_v2.DeleteLoadBalancerInput{
				LoadBalancerArn: aws_v2.String(arn),
			})
		if err != nil {
			ts.lg.Warn("failed to remove ELBv2",
				zap.String("elb-v2-arn", arn),
				zap.Error(err),
			)
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) {
				if strings.Contains(apiErr.ErrorCode(), "NotFound") {
					ts.cfg.DeletedResources[arn] = "ELBV2"
					ts.cfg.Sync()
				} else {
					deleted = false
				}
			} else {
				deleted = false
			}
			continue
		}

		ts.lg.Info("removed ELBv2", zap.String("elb-arn", arn), zap.Error(err))
		ts.cfg.DeletedResources[arn] = "ELBV2"
	}

	if deleted {
		ts.lg.Info("deleted ELBv2 for the VPC", zap.String("vpc-id", ts.cfg.VPC.ID))
		return nil
	}
	return errors.New("failed to delete all ELB v2")
}
