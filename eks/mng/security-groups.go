package mng

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

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/mng/security-groups.go for CloudFormation based workflow

// "[sig-network] Networking Granular Checks" in "test/e2e/network/dns.go"
// requires "e2enetwork.EndpointUDPPort/EndpointHTTPPort", 8081 and 8080
// just open all for now...
// TODO: restrict ports

// AWS::EC2::SecurityGroup
func (ts *tester) authorizeSecurityGroups(name string) error {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot authorize ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return fmt.Errorf("MNG[%q] security group ID not found; cannot authorize ingress/egress security group", name)
	}
	ts.cfg.Logger.Info("authorizing security group",
		zap.String("mng-name", name),
		zap.String("api-server-node-security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID),
		zap.String("managed-node-group-security-group-id", cur.RemoteAccessSecurityGroupID),
	)

	// allow node to communicate with each other
	ts.cfg.Logger.Info("authorizing IngressWithinManagedNodeGroupSecurityGroup", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err := ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("-1"),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized IngressWithinManagedNodeGroupSecurityGroup")

	// allow pods to communicate with the cluster API Server
	ts.cfg.Logger.Info("authorizing Ingress443FromNGtoCP", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized Ingress443FromNGtoCP")

	// allow pods running extension API servers on port 443
	// to receive communication from cluster control plane
	ts.cfg.Logger.Info("authorizing Ingress443FromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// egress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized Ingress443FromCPtoNG")

	// allow the cluster control plane to communicate with pods running extension API servers on port 443
	ts.cfg.Logger.Info("authorizing Egress443FromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized Egress443FromCPtoNG")

	// allow worker Kubelets and pods to receive communication from the cluster control plane
	ts.cfg.Logger.Info("authorizing IngressAllFromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized IngressAllFromCPtoNG")

	// allow the cluster control plane to communicate with worker Kubelet and pods
	ts.cfg.Logger.Info("authorizing EgressAllFromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized EgressAllFromCPtoNG")

	ts.cfg.Logger.Info("authorizing Ingress22ForSSH", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized Ingress22ForSSH")

	ts.cfg.Logger.Info("authorizing IngressForGuestBook", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized IngressForGuestBook")

	ts.cfg.Logger.Info("authorizing EgressForGuestBook", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized EgressForGuestBook")

	ts.cfg.Logger.Info("authorizing IngressForNodePortConformance", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.AuthorizeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "Duplicate") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("authorized IngressForNodePortConformance")

	ts.cfg.Logger.Info("authorized security group")
	return nil
}

func (ts *tester) revokeSecurityGroups(name string) (err error) {
	cur, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[name]
	if !ok {
		return fmt.Errorf("MNGs[%q] not found; cannot revoke ingress/egress security group", name)
	}
	if cur.RemoteAccessSecurityGroupID == "" {
		return fmt.Errorf("MNG[%q] security group ID not found; cannot revoke ingress/egress security group", name)
	}
	ts.cfg.Logger.Info("revoking security group",
		zap.String("mng-name", name),
		zap.String("api-server-node-security-group-id", ts.cfg.EKSConfig.VPC.SecurityGroupID),
		zap.String("managed-node-group-security-group-id", cur.RemoteAccessSecurityGroupID),
	)

	// allow node to communicate with each other
	ts.cfg.Logger.Info("revoking IngressWithinNodeGroupSecurityGroup", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),

			IpPermissions: []aws_ec2_v2_types.IpPermission{
				{
					IpProtocol: aws_v2.String("-1"),
					UserIdGroupPairs: []aws_ec2_v2_types.UserIdGroupPair{
						{
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked IngressWithinNodeGroupSecurityGroup")

	// allow pods to communicate with the cluster API Server
	ts.cfg.Logger.Info("revoking Ingress443FromNGtoCP", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked Ingress443FromNGtoCP")

	// allow pods running extension API servers on port 443
	// to receive communication from cluster control plane
	ts.cfg.Logger.Info("revoking Ingress443FromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// egress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked Ingress443FromCPtoNG")

	// allow the cluster control plane to communicate with pods running extension API servers on port 443
	ts.cfg.Logger.Info("revoking Egress443FromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked Egress443FromCPtoNG")

	// allow worker Kubelets and pods to receive communication from the cluster control plane
	ts.cfg.Logger.Info("revoking IngressAllFromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked IngressAllFromCPtoNG")

	// allow the cluster control plane to communicate with worker Kubelet and pods
	ts.cfg.Logger.Info("revoking EgressAllFromCPtoNG", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
							GroupId:     aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked EgressAllFromCPtoNG")

	ts.cfg.Logger.Info("revoking Ingress22ForSSH", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked Ingress22ForSSH")

	ts.cfg.Logger.Info("revoking IngressForGuestBook", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked IngressForGuestBook")

	ts.cfg.Logger.Info("revoking EgressForGuestBook", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked EgressForGuestBook")

	ts.cfg.Logger.Info("revoking IngressForNodePortConformance", zap.String("sg-id", cur.RemoteAccessSecurityGroupID))
	_, err = ts.cfg.EC2APIV2.RevokeSecurityGroupIngress(
		context.Background(),
		&aws_ec2_v2.RevokeSecurityGroupIngressInput{
			// ingress target
			GroupId: aws_v2.String(cur.RemoteAccessSecurityGroupID),
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
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				err = nil
			}
		}
		if err != nil {
			return err
		}
	}
	ts.cfg.Logger.Info("revoked IngressForNodePortConformance")

	ts.cfg.Logger.Info("revoked security group")
	return nil
}
