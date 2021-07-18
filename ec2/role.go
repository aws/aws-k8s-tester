package ec2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aws_iam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_iam_v2 "github.com/aws/aws-sdk-go-v2/service/iam"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

// see https://github.com/aws/aws-k8s-tester/blob/v1.6.0/eks/cluster/role.go for CloudFormation based workflow

func (ts *Tester) createRole() error {
	fmt.Print(ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.Colorize("[light_green]createRole [default](%q)\n"), ts.cfg.ConfigPath)

	if !ts.cfg.Role.Create {
		ts.lg.Info("Role.Create false; skipping creation")
		return aws_iam.ValidateV2(
			ts.lg,
			ts.iamAPIV2,
			ts.cfg.Role.Name,
			[]string{"ec2.amazonaws.com"},
			[]string{
				"arn:aws:iam::aws:policy/AmazonEC2FullAccess",
				"arn:aws:iam::aws:policy/AmazonSSMFullAccess",
				"arn:aws:iam::aws:policy/AmazonS3FullAccess",
			},
		)
	}
	if ts.cfg.Role.ARN != "" {
		ts.lg.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.Role.Name == "" {
		return errors.New("cannot create a cluster role with an empty Role.Name")
	}

	if err := ts._createRole(); err != nil {
		return err
	}
	if err := ts.createPolicy(); err != nil {
		return err
	}
	if err := ts.attachPolicy(); err != nil {
		return err
	}
	if err := ts.createInstanceProfile(); err != nil {
		return err
	}

	ts.lg.Info("created a new role and attached policy",
		zap.String("role-arn", ts.cfg.Role.ARN),
		zap.String("role-name", ts.cfg.Role.Name),
	)
	return nil
}

func (ts *Tester) deleteRole() error {
	fmt.Print(ts.cfg.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.Colorize("[light_blue]deleteRole [default](%q)\n"), ts.cfg.ConfigPath)

	if !ts.cfg.Role.Create {
		ts.lg.Info("Role.Create false; skipping deletion")
		return nil
	}

	var errs []string
	if err := ts.deleteInstanceProfile(); err != nil {
		ts.lg.Warn("failed to delete instance profile", zap.Error(err))
	}
	if err := ts.detachPolicy(); err != nil {
		ts.lg.Warn("failed to detach policy", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deletePolicy(); err != nil {
		ts.lg.Warn("failed to delete policy", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts._deleteRole(); err != nil {
		ts.lg.Warn("failed to delete role", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteInstanceProfile(); err != nil {
		ts.lg.Warn("failed to delete instance profile", zap.Error(err))
	}

	if len(errs) == 0 {
		ts.lg.Info("successfully deleted role",
			zap.String("role-arn", ts.cfg.Role.ARN),
			zap.String("role-name", ts.cfg.Role.Name),
		)
		return nil
	}
	return errors.New(strings.Join(errs, ","))
}

func (ts *Tester) _createRole() error {
	ts.lg.Info("creating role", zap.String("name", ts.cfg.Role.Name))
	out, err := ts.iamAPIV2.CreateRole(
		context.Background(),
		&aws_iam_v2.CreateRoleInput{
			RoleName:                 aws_v2.String(ts.cfg.Role.Name),
			Path:                     aws_v2.String("/"),
			AssumeRolePolicyDocument: aws_v2.String(createAssumeRolePolicyDocument(ts.cfg.Role.ServicePrincipals)),
		},
	)
	if err != nil {
		return err
	}

	ts.lg.Info("created role")
	ts.cfg.Role.ARN = aws_v2.ToString(out.Role.Arn)
	ts.cfg.Sync()
	return nil
}

func (ts *Tester) _deleteRole() error {
	ts.lg.Info("deleting role", zap.String("name", ts.cfg.Role.Name))
	if _, ok := ts.cfg.DeletedResources[ts.cfg.Role.Name]; ok {
		return nil
	}

	_, err := ts.iamAPIV2.DeleteRole(
		context.Background(),
		&aws_iam_v2.DeleteRoleInput{
			RoleName: aws_v2.String(ts.cfg.Role.Name),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete cluster role", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.Role.Name] = "Role.Name"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.lg.Info("deleted role")
	ts.cfg.DeletedResources[ts.cfg.Role.Name] = "Role.Name"
	ts.cfg.Sync()
	return nil
}

// https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
// https://github.com/aws/eks-charts/tree/master/stable/appmesh-controller
func (ts *Tester) createPolicy() error {
	if ts.cfg.Role.PolicyName == "" {
		return errors.New("emtpy PolicyName")
	}
	ts.lg.Info("creating policy", zap.String("name", ts.cfg.Role.PolicyName))
	pout, err := ts.iamAPIV2.CreatePolicy(
		context.Background(),
		&aws_iam_v2.CreatePolicyInput{
			PolicyName:     aws_v2.String(ts.cfg.Role.PolicyName),
			PolicyDocument: aws_v2.String(createRolePolicyDocument(ts.cfg.Partition, ts.cfg.S3.BucketName)),
		},
	)
	if err != nil {
		return err
	}

	ts.lg.Info("created policy")
	ts.cfg.Role.PolicyARN = aws_v2.ToString(pout.Policy.Arn)
	ts.cfg.Sync()
	return nil
}

func (ts *Tester) deletePolicy() error {
	ts.lg.Info("deleting policy")
	if _, ok := ts.cfg.DeletedResources[ts.cfg.Role.PolicyARN]; ok {
		return nil
	}

	_, err := ts.iamAPIV2.DeletePolicy(
		context.Background(),
		&aws_iam_v2.DeletePolicyInput{
			PolicyArn: aws_v2.String(ts.cfg.Role.PolicyARN),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to delete policy", zap.Error(err))
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.cfg.DeletedResources[ts.cfg.Role.PolicyARN] = "Role.PolicyARN"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.lg.Info("deleted policy")
	ts.cfg.DeletedResources[ts.cfg.Role.PolicyARN] = "Role.PolicyARN"
	ts.cfg.Sync()
	return nil
}

func (ts *Tester) attachPolicy() error {
	ts.lg.Info("attaching policies")

	_, err := ts.iamAPIV2.AttachRolePolicy(
		context.Background(),
		&aws_iam_v2.AttachRolePolicyInput{
			RoleName:  aws_v2.String(ts.cfg.Role.Name),
			PolicyArn: aws_v2.String(ts.cfg.Role.PolicyARN),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to attach policy", zap.String("arn", ts.cfg.Role.PolicyARN), zap.Error(err))
		return err
	}
	ts.lg.Info("attached policy arn", zap.String("policy-arn", ts.cfg.Role.PolicyARN))

	for _, arn := range ts.cfg.Role.ManagedPolicyARNs {
		time.Sleep(3 * time.Second)
		ts.lg.Info("attaching managed policy arn", zap.String("arn", arn))
		_, err := ts.iamAPIV2.AttachRolePolicy(
			context.Background(),
			&aws_iam_v2.AttachRolePolicyInput{
				RoleName:  aws_v2.String(ts.cfg.Role.Name),
				PolicyArn: aws_v2.String(arn),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to attach policy", zap.String("arn", arn), zap.Error(err))
			return err
		}
	}

	ts.lg.Info("attached policies")
	return nil
}

func (ts *Tester) detachPolicy() error {
	ts.lg.Info("detaching policies")

	_, err := ts.iamAPIV2.DetachRolePolicy(
		context.Background(),
		&aws_iam_v2.DetachRolePolicyInput{
			RoleName:  aws_v2.String(ts.cfg.Role.Name),
			PolicyArn: aws_v2.String(ts.cfg.Role.PolicyARN),
		},
	)
	if err != nil {
		ts.lg.Warn("failed to detach policy", zap.String("arn", ts.cfg.Role.PolicyARN), zap.Error(err))
		return err
	}
	for _, arn := range ts.cfg.Role.ManagedPolicyARNs {
		time.Sleep(3 * time.Second)
		_, err := ts.iamAPIV2.DetachRolePolicy(
			context.Background(),
			&aws_iam_v2.DetachRolePolicyInput{
				RoleName:  aws_v2.String(ts.cfg.Role.Name),
				PolicyArn: aws_v2.String(arn),
			},
		)
		if err != nil {
			ts.lg.Warn("failed to detach policy", zap.String("arn", arn), zap.Error(err))
			return err
		}
	}

	ts.lg.Info("detached policies")
	return nil
}

func createAssumeRolePolicyDocument(sps []string) string {
	p := aws_iam.PolicyDocument{
		Version:   "2012-10-17",
		Statement: createStatementEntriesForAssumeRole(sps),
	}
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func createRolePolicyDocument(partition string, bucketName string) string {
	p := aws_iam.PolicyDocument{
		Version:   "2012-10-17",
		Statement: createStatementEntriesForRolePolicyDocument(partition, bucketName),
	}
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func createStatementEntriesForAssumeRole(sps []string) []aws_iam.StatementEntry {
	return []aws_iam.StatementEntry{
		{
			Effect: "Allow",
			Principal: &aws_iam.PrincipalEntry{
				Service: sps,
			},
			Action: []string{
				"sts:AssumeRole",
			},
		},
	}
}

// TODO: update based on add-on setups
// https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html
// https://github.com/kubernetes-sigs/aws-alb-ingress-controller/blob/master/docs/examples/iam-policy.json
// https://github.com/aws/eks-charts/tree/master/stable/appmesh-controller
func createStatementEntriesForRolePolicyDocument(partition string, bucketName string) []aws_iam.StatementEntry {
	return []aws_iam.StatementEntry{
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"acm:DescribeCertificate",
				"acm:ListCertificates",
				"acm:GetCertificate",
			},
		},
		// arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"ec2:AttachVolume",
				"ec2:AuthorizeSecurityGroupIngress",
				"ec2:CreateSecurityGroup",
				"ec2:CreateSnapshot",
				"ec2:CreateTags",
				"ec2:CreateVolume",
				"ec2:DeleteSecurityGroup",
				"ec2:DeleteSnapshot",
				"ec2:DeleteTags",
				"ec2:DeleteVolume",
				"ec2:DescribeAccountAttributes",
				"ec2:DescribeAddresses",
				"ec2:DescribeInstanceStatus",
				"ec2:DescribeInstances",
				"ec2:DescribeInternetGateways",
				"ec2:DescribeNetworkInterfaces",
				"ec2:DescribeRouteTables",
				"ec2:DescribeSecurityGroups",
				"ec2:DescribeSnapshots",
				"ec2:DescribeSubnets",
				"ec2:DescribeTags",
				"ec2:DescribeVolumes",
				"ec2:DescribeVolumes",
				"ec2:DescribeVolumesModifications",
				"ec2:DescribeVpcs",
				"ec2:DetachVolume",
				"ec2:ModifyInstanceAttribute",
				"ec2:ModifyNetworkInterfaceAttribute",
				"ec2:RevokeSecurityGroupIngress",
				"eks:DescribeCluster",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"elasticloadbalancing:AddListenerCertificates",
				"elasticloadbalancing:AddTags",
				"elasticloadbalancing:CreateListener",
				"elasticloadbalancing:CreateLoadBalancer",
				"elasticloadbalancing:CreateRule",
				"elasticloadbalancing:CreateTargetGroup",
				"elasticloadbalancing:DeleteListener",
				"elasticloadbalancing:DeleteLoadBalancer",
				"elasticloadbalancing:DeleteRule",
				"elasticloadbalancing:DeleteTargetGroup",
				"elasticloadbalancing:DeregisterTargets",
				"elasticloadbalancing:DescribeListenerCertificates",
				"elasticloadbalancing:DescribeListeners",
				"elasticloadbalancing:DescribeLoadBalancers",
				"elasticloadbalancing:DescribeLoadBalancerAttributes",
				"elasticloadbalancing:DescribeRules",
				"elasticloadbalancing:DescribeSSLPolicies",
				"elasticloadbalancing:DescribeTags",
				"elasticloadbalancing:DescribeTargetGroups",
				"elasticloadbalancing:DescribeTargetGroupAttributes",
				"elasticloadbalancing:DescribeTargetHealth",
				"elasticloadbalancing:ModifyListener",
				"elasticloadbalancing:ModifyLoadBalancerAttributes",
				"elasticloadbalancing:ModifyRule",
				"elasticloadbalancing:ModifyTargetGroup",
				"elasticloadbalancing:ModifyTargetGroupAttributes",
				"elasticloadbalancing:RegisterTargets",
				"elasticloadbalancing:RemoveListenerCertificates",
				"elasticloadbalancing:RemoveTags",
				"elasticloadbalancing:SetIpAddressType",
				"elasticloadbalancing:SetSecurityGroups",
				"elasticloadbalancing:SetSubnets",
				"elasticloadbalancing:SetWebACL",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"iam:CreateServiceLinkedRole",
				"iam:GetServerCertificate",
				"iam:ListServerCertificates",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"cognito-idp:DescribeUserPoolClient",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"waf-regional:GetWebACLForResource",
				"waf-regional:GetWebACL",
				"waf-regional:AssociateWebACL",
				"waf-regional:DisassociateWebACL",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"tag:GetResources",
				"tag:TagResources",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"waf:GetWebACL",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"wafv2:GetWebACL",
				"wafv2:GetWebACLForResource",
				"wafv2:AssociateWebACL",
				"wafv2:DisassociateWebACL",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"shield:DescribeProtection",
				"shield:GetSubscriptionState",
				"shield:DeleteProtection",
				"shield:CreateProtection",
				"shield:DescribeSubscription",
				"shield:ListProtections",
			},
		},
		{
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"appmesh:*",
				"servicediscovery:CreateService",
				"servicediscovery:GetService",
				"servicediscovery:RegisterInstance",
				"servicediscovery:DeregisterInstance",
				"servicediscovery:ListInstances",
				"servicediscovery:ListNamespaces",
				"servicediscovery:ListServices",
				"route53:GetHealthCheck",
				"route53:CreateHealthCheck",
				"route53:UpdateHealthCheck",
				"route53:ChangeResourceRecordSets",
				"route53:DeleteHealthCheck",
			},
		},
		{ // for fluentd add-on
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"logs:CreateLogGroup",
				"logs:CreateLogStream",
				"logs:DescribeLogGroups",
				"logs:DescribeLogStreams",
				"logs:PutLogEvents",
			},
		},
		{ // for cluster autoscaler
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"autoscaling:DescribeAutoScalingGroups",
				"autoscaling:DescribeAutoScalingInstances",
				"autoscaling:DescribeLaunchConfigurations",
				"autoscaling:DescribeTags",
				"autoscaling:SetDesiredCapacity",
				"autoscaling:TerminateInstanceInAutoScalingGroup",
				"ec2:DescribeLaunchTemplateVersions",
			},
		},
		{ // for artifact uploads from worker nodes
			Effect:   "Allow",
			Resource: fmt.Sprintf("arn:%s:s3:::%s/*", partition, bucketName),
			Action: []string{
				"s3:ListBucket",
				"s3:GetObject",
				"s3:PutObject",
			},
		},
		{ // arn:aws:iam::aws:policy/AmazonS3FullAccess
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"s3:*",
			},
		},
		{ // arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
			Effect:   "Allow",
			Resource: "*",
			Action: []string{
				"ecr:GetAuthorizationToken",
				"ecr:BatchCheckLayerAvailability",
				"ecr:GetDownloadUrlForLayer",
				"ecr:GetRepositoryPolicy",
				"ecr:DescribeRepositories",
				"ecr:ListImages",
				"ecr:DescribeImages",
				"ecr:BatchGetImage",
				"ecr:GetLifecyclePolicy",
				"ecr:GetLifecyclePolicyPreview",
				"ecr:ListTagsForResource",
				"ecr:DescribeImageScanFindings",
			},
		},
	}
}

func (ts *Tester) createInstanceProfile() error {
	ts.lg.Info("creating instance profile")
	out, err := ts.iamAPIV2.CreateInstanceProfile(
		context.Background(),
		&aws_iam_v2.CreateInstanceProfileInput{
			InstanceProfileName: aws_v2.String(ts.cfg.Role.InstanceProfileName),
			Path:                aws_v2.String("/"),
		},
	)
	if err != nil {
		return err
	}
	ts.cfg.Role.InstanceProfileARN = aws_v2.ToString(out.InstanceProfile.Arn)
	ts.cfg.Sync()

	_, err = ts.iamAPIV2.AddRoleToInstanceProfile(
		context.Background(),
		&aws_iam_v2.AddRoleToInstanceProfileInput{
			InstanceProfileName: aws_v2.String(ts.cfg.Role.InstanceProfileName),
			RoleName:            aws_v2.String(ts.cfg.Role.Name),
		},
	)
	if err != nil {
		return err
	}

	ts.lg.Info("created instance profile")
	return nil
}

func (ts *Tester) deleteInstanceProfile() error {
	ts.lg.Info("deleting instance profile")
	if _, ok := ts.cfg.DeletedResources[ts.cfg.Role.InstanceProfileName]; ok {
		return nil
	}

	_, err := ts.iamAPIV2.RemoveRoleFromInstanceProfile(
		context.Background(),
		&aws_iam_v2.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: aws_v2.String(ts.cfg.Role.InstanceProfileName),
			RoleName:            aws_v2.String(ts.cfg.Role.Name),
		},
	)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") || strings.Contains(apiErr.ErrorCode(), "NotFoundException") {
				ts.cfg.DeletedResources[ts.cfg.Role.InstanceProfileName] = "Role.InstanceProfileName"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	_, err = ts.iamAPIV2.DeleteInstanceProfile(
		context.Background(),
		&aws_iam_v2.DeleteInstanceProfileInput{
			InstanceProfileName: aws_v2.String(ts.cfg.Role.InstanceProfileName),
		},
	)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") || strings.Contains(apiErr.ErrorCode(), "NotFoundException") {
				ts.cfg.DeletedResources[ts.cfg.Role.InstanceProfileName] = "Role.InstanceProfileName"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	ts.cfg.DeletedResources[ts.cfg.Role.InstanceProfileName] = "Role.InstanceProfileName"
	ts.cfg.Sync()
	ts.lg.Info("deleted instance profile")

	return nil
}
