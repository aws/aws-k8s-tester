package ng

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	aws_ec2_v2_types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/ng/security-groups.go for CloudFormation based workflow

// "[sig-network] Networking Granular Checks" in "test/e2e/network/dns.go"
// requires "e2enetwork.EndpointUDPPort/EndpointHTTPPort", 8081 and 8080
// just open all for now...
// TODO: restrict ports

// AWS::EC2::SecurityGroup
func (ts *tester) createSecurityGroups() error {
	ts.cfg.Logger.Info("creating security group")

	sout, err := ts.cfg.EC2APIV2.CreateSecurityGroup(
		context.Background(),
		&aws_ec2_v2.CreateSecurityGroupInput{
			GroupName:   aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupName),
			Description: aws_v2.String("Security group for all nodes in the cluster"),
			VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
			TagSpecifications: []aws_ec2_v2_types.TagSpecification{
				{
					ResourceType: aws_ec2_v2_types.ResourceTypeSecurityGroup,
					Tags: []aws_ec2_v2_types.Tag{
						{
							Key:   aws_v2.String(fmt.Sprintf("kubernetes.io/cluster/%s", ts.cfg.EKSConfig.Name)),
							Value: aws_v2.String("owned"),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create security group", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID = aws_v2.ToString(sout.GroupId)
	ts.cfg.EKSConfig.Sync()
	ts.cfg.Logger.Info("created security group", zap.String("security-group-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))

	return nil
}

func (ts *tester) deleteSecurityGroups() (err error) {
	ts.cfg.Logger.Info("deleting security group")
	if ts.cfg.EKSConfig.VPC.ID == "" {
		return nil
	}
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID]; ok {
		return nil
	}

	_, err = ts.cfg.EC2APIV2.DeleteSecurityGroup(
		context.Background(),
		&aws_ec2_v2.DeleteSecurityGroupInput{
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to delete security group", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID] = "VPC.NodeGroupSecurityGroupID"
				ts.cfg.EKSConfig.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.Logger.Info("deleted security group")
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID] = "VPC.NodeGroupSecurityGroupID"
	ts.cfg.EKSConfig.Sync()

	return nil
}

func (ts *tester) authorizeSecurityGroup() error {
	ts.cfg.Logger.Info("authorizing security group",
		zap.String("api-server-node-security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID),
		zap.String("node-group-security-group-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
	)

	// allow node to communicate with each other
	ts.cfg.Logger.Info("authorizing IngressWithinNodeGroupSecurityGroup", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err := ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("-1"),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("allow node to communicate with each other"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized IngressWithinNodeGroupSecurityGroup")

	// allow pods to communicate with the cluster API Server
	ts.cfg.Logger.Info("authorizing Ingress443FromNGtoCP", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("allow pods to communicate with the cluster API Server"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized Ingress443FromNGtoCP")

	// allow pods running extension API servers on port 443
	// to receive communication from cluster control plane
	ts.cfg.Logger.Info("authorizing Ingress443FromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
							Description: aws_v2.String("receive communication from cluster control plane"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized Ingress443FromCPtoNG")

	// allow the cluster control plane to communicate with pods running extension API servers on port 443
	ts.cfg.Logger.Info("authorizing Egress443FromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("communicate with pods running extension API servers on port 443"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized Egress443FromCPtoNG")

	// allow worker Kubelets and pods to receive communication from the cluster control plane
	ts.cfg.Logger.Info("authorizing IngressAllFromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(0),
					ToPort:     aws_v2.Int32(65535),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
							Description: aws_v2.String("receive communication from the cluster control plane"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized IngressAllFromCPtoNG")

	// allow the cluster control plane to communicate with worker Kubelet and pods
	ts.cfg.Logger.Info("authorizing EgressAllFromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(0),
					ToPort:     aws_v2.Int32(65535),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("communicate with worker Kubelet and pods"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized EgressAllFromCPtoNG")

	ts.cfg.Logger.Info("authorizing Ingress22ForSSH", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(22),
					ToPort:   aws_v2.Int32(22),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized Ingress22ForSSH")

	ts.cfg.Logger.Info("authorizing IngressForGuestBook", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(1),
					ToPort:   aws_v2.Int32(10000),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized IngressForGuestBook")

	ts.cfg.Logger.Info("authorizing EgressForGuestBook", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(1),
					ToPort:     aws_v2.Int32(10000),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized EgressForGuestBook")

	ts.cfg.Logger.Info("authorizing IngressForNodePortConformance", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(1),
					ToPort:   aws_v2.Int32(32767),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to authorize ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("authorized IngressForNodePortConformance")

	ts.cfg.Logger.Info("authorized security group")
	return nil
}

func (ts *tester) revokeSecurityGroups() (err error) {
	ts.cfg.Logger.Info("revoking security group")
	if ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID == "" {
		return nil
	}

	// allow node to communicate with each other
	ts.cfg.Logger.Info("revoking IngressWithinNodeGroupSecurityGroup", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("-1"),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("allow node to communicate with each other"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked IngressWithinNodeGroupSecurityGroup")

	// allow pods to communicate with the cluster API Server
	ts.cfg.Logger.Info("revoking Ingress443FromNGtoCP", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("allow pods to communicate with the cluster API Server"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked Ingress443FromNGtoCP")

	// allow pods running extension API servers on port 443
	// to receive communication from cluster control plane
	ts.cfg.Logger.Info("revoking Ingress443FromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
							Description: aws_v2.String("receive communication from cluster control plane"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked Ingress443FromCPtoNG")

	// allow the cluster control plane to communicate with pods running extension API servers on port 443
	ts.cfg.Logger.Info("revoking Egress443FromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(443),
					ToPort:     aws_v2.Int32(443),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("communicate with pods running extension API servers on port 443"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked Egress443FromCPtoNG")

	// allow worker Kubelets and pods to receive communication from the cluster control plane
	ts.cfg.Logger.Info("revoking IngressAllFromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(0),
					ToPort:     aws_v2.Int32(65535),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
							Description: aws_v2.String("receive communication from the cluster control plane"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked IngressAllFromCPtoNG")

	// allow the cluster control plane to communicate with worker Kubelet and pods
	ts.cfg.Logger.Info("revoking EgressAllFromCPtoNG", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(0),
					ToPort:     aws_v2.Int32(65535),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
							Description: aws_v2.String("communicate with worker Kubelet and pods"),
							VpcId:       aws_v2.String(ts.cfg.EKSConfig.VPC.ID),
						},
					},
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked EgressAllFromCPtoNG")

	ts.cfg.Logger.Info("revoking Ingress22ForSSH", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(22),
					ToPort:   aws_v2.Int32(22),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked Ingress22ForSSH")

	ts.cfg.Logger.Info("revoking IngressForGuestBook", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(1),
					ToPort:   aws_v2.Int32(10000),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked IngressForGuestBook")

	ts.cfg.Logger.Info("revoking EgressForGuestBook", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupEgress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupEgressInput{
			// egress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.SecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					FromPort:   aws_v2.Int32(1),
					ToPort:     aws_v2.Int32(10000),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke egress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked EgressForGuestBook")

	ts.cfg.Logger.Info("revoking IngressForNodePortConformance", zap.String("sg-id", ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(ts.cfg.EKSConfig.VPC.NodeGroupSecurityGroupID),
			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("tcp"),
					IpRanges: []aws_ec2_v2_types.IpRange{
						{
							CidrIp: aws_v2.String("0.0.0.0/0"),
						},
					},
					FromPort: aws_v2.Int32(1),
					ToPort:   aws_v2.Int32(32767),
				},
			},
		},
	)
	if err != nil {
		ts.cfg.Logger.Warn("failed to revoke ingress", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("revoked IngressForNodePortConformance")

	ts.cfg.Logger.Info("revoked security group")
	return nil
}
