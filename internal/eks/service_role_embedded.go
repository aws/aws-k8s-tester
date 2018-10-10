package eks

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) createAWSServiceRoleForAmazonEKS() error {
	if md.cfg.ClusterState.ServiceRoleWithPolicyName == "" {
		return errors.New("cannot create empty service role")
	}

	now := time.Now().UTC()

	op1, err := md.im.CreateRole(&iam.CreateRoleInput{
		RoleName:                 aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
		AssumeRolePolicyDocument: aws.String(serviceRolePolicyDoc),
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusRoleCreated = true
	md.cfg.Sync()

	// check if it has been created
	op2, err := md.im.GetRole(&iam.GetRoleInput{
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

	_, err := md.im.DeleteRole(&iam.DeleteRoleInput{
		RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName),
	})
	if err != nil && !isIAMRoleDeletedGoClient(err) {
		return err
	}

	// check if it has been deleted
	_, err = md.im.GetRole(&iam.GetRoleInput{RoleName: aws.String(md.cfg.ClusterState.ServiceRoleWithPolicyName)})
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
		_, err := md.im.AttachRolePolicy(&iam.AttachRolePolicyInput{
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
		_, err := md.im.DetachRolePolicy(&iam.DetachRolePolicyInput{
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
