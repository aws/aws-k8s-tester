package eks

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

const (
	serviceRolePolicyDocProd = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Service": "eks.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		}
	]
}`
	serviceRolePolicyDocBeta = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Service": [
					"eks-dev.aws.internal",
					"eks-beta-pdx.aws.internal",
					"eks.amazonaws.com"
				]
			},
			"Action": "sts:AssumeRole"
		}
	]
}`
)

var stageToServiceRole = map[string]string{
	"prod": serviceRolePolicyDocProd,
	"https://api.beta.us-west-2.wesley.amazonaws.com": serviceRolePolicyDocBeta,
}

func (md *embedded) createAWSServiceRoleForAmazonEKS() error {
	if md.cfg.ClusterState.ServiceRoleWithPolicyName == "" {
		return errors.New("cannot create empty service role")
	}

	policyDoc := stageToServiceRole["prod"]
	if md.cfg.EKSResolverURL != "" {
		var ok bool
		policyDoc, ok = stageToServiceRole[md.cfg.EKSResolverURL]
		if !ok {
			return fmt.Errorf("service role policy for %q not found", md.cfg.EKSResolverURL)
		}
	}

	now := time.Now().UTC()

	op1, err := md.iam.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
		AssumeRolePolicyDocument: aws.String(policyDoc),
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusRoleCreated = true
	md.cfg.Sync()

	// check if it has been created
	op2, err := md.iam.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
	})
	if err != nil {
		return err
	}
	if *op1.Role.Arn != *op2.Role.Arn {
		return fmt.Errorf("'aws iam create-role' Role.Arn %q != 'aws iam get-role' Role.Arn %q", *op1.Role.Arn, *op2.Role.Arn)
	}
	md.cfg.ClusterState.ServiceRoleWithPolicyARN = *op1.Role.Arn

	md.lg.Info("created IAM role for AWSServiceRoleForAmazonEKS",
		zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteAWSServiceRoleForAmazonEKS() error {
	if !md.cfg.ClusterState.StatusRoleCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusRoleCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterState.ServiceRoleWithPolicyName == "" {
		return errors.New("cannot delete empty service role")
	}

	now := time.Now().UTC()

	_, err := md.iam.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
	})
	if err != nil && !isIAMRoleDeletedGoClient(err) {
		return err
	}

	// check if it has been deleted
	_, err = md.iam.GetRole(&iam.GetRoleInput{RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName)})
	if !isIAMRoleDeletedGoClient(err) {
		return fmt.Errorf("%s still exists after 'DeleteRole' call (error %v)", md.cfg.ClusterState.ServiceRoleWithPolicyName, err)
	}

	md.cfg.ClusterState.ServiceRoleWithPolicyARN = ""
	md.lg.Info("deleted IAM role for AWSServiceRoleForAmazonEKS",
		zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) attachPolicyForAWSServiceRoleForAmazonEKS() error {
	if md.cfg.ClusterState.ServiceRoleWithPolicyName == "" {
		return errors.New("cannot attach to empty service role")
	}

	now := time.Now().UTC()

	for _, pv := range md.cfg.ClusterState.ServiceRolePolicies {
		_, err := md.iam.AttachRolePolicy(&iam.AttachRolePolicyInput{
			RoleName:  aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
			PolicyArn: aws.String(pv),
		})
		if err != nil {
			return err
		}

		md.cfg.ClusterState.StatusPolicyAttached = true
		md.cfg.Sync()

		md.lg.Info("attached IAM role policy for AWSServiceRoleForAmazonEKS",
			zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
			zap.String("service-role-arn", md.cfg.ClusterState.ServiceRoleWithPolicyARN),
			zap.String("service-role-policy-arn", pv),
		)
	}

	md.lg.Info("attached policies to AWSServiceRoleForAmazonEKS",
		zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}

func (md *embedded) detachPolicyForAWSServiceRoleForAmazonEKS() error {
	if !md.cfg.ClusterState.StatusPolicyAttached {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusPolicyAttached = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterState.ServiceRoleWithPolicyName == "" {
		return errors.New("cannot detach from empty service role")
	}

	now := time.Now().UTC()

	for _, pv := range md.cfg.ClusterState.ServiceRolePolicies {
		_, err := md.iam.DetachRolePolicy(&iam.DetachRolePolicyInput{
			RoleName:  aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
			PolicyArn: aws.String(pv),
		})
		if err != nil && !isIAMRoleDeletedGoClient(err) {
			return err
		}
		md.lg.Info("detached IAM role policy for AWSServiceRoleForAmazonEKS",
			zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
			zap.String("service-role-arn", md.cfg.ClusterState.ServiceRoleWithPolicyARN),
			zap.String("service-role-policy-arn", pv),
		)
	}
	md.lg.Info("detached policies to AWSServiceRoleForAmazonEKS",
		zap.String("service-role-name", md.cfg.ClusterState.ServiceRoleWithPolicyName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return nil
}

// isIAMRoleDeletedGoClient returns true if error from IAM API call
// indicates that the role has already been deleted or does not exist.
func isIAMRoleDeletedGoClient(err error) bool {
	if err == nil {
		return false
	}
	// NoSuchEntity: The user with name aws-k8s-tester-155479CED0B80EE801-SERVICE-ROLE-POLICY cannot be found.
	// NoSuchEntity: The role with name aws-k8s-tester-20180918-TESTID-4RHA3tT-SERVICE-ROLE cannot be found.
	// TODO: use aweerr.Code
	return strings.Contains(err.Error(), "cannot be found")
}
