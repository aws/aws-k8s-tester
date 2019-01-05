package csi

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"go.uber.org/zap"
)

const (
	assumeRoleDocument = `{
  "Version": "2012-10-17",
  "Statement": {
    "Effect": "Allow",
    "Principal": {"Service": "ec2.amazonaws.com"},
    "Action": "sts:AssumeRole"
  }
}`

	policyDocument = `{
   "Version": "2012-10-17",
   "Statement": [
       {
           "Action": "ec2:*",
           "Effect": "Allow",
           "Resource": "*"
       },
       {
           "Effect": "Allow",
           "Action": "elasticloadbalancing:*",
           "Resource": "*"
       },
       {
           "Effect": "Allow",
           "Action": "cloudwatch:*",
           "Resource": "*"
       },
       {
           "Effect": "Allow",
           "Action": "autoscaling:*",
           "Resource": "*"
       },
       {
           "Effect": "Allow",
           "Action": "iam:CreateServiceLinkedRole",
           "Resource": "*",
           "Condition": {
               "StringEquals": {
                   "iam:AWSServiceName": [
                       "autoscaling.amazonaws.com",
                       "ec2scheduled.amazonaws.com",
                       "elasticloadbalancing.amazonaws.com",
                       "spot.amazonaws.com",
                       "spotfleet.amazonaws.com"
                   ]
               }
           }
       }
   ]
}`
)

type iamResources struct {
	svc             *iam.IAM
	instanceProfile *iamResource
	policy          *iamResource
	role            *iamResource

	lg *zap.Logger
}

type iamResource struct {
	name string
	arn  string
}

// awsRegion must be a valid AWS region for ec2 instances.
func createIAM(awsRegion string) (*iam.IAM, error) {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(awsRegion)})
	if err != nil {
		return nil, fmt.Errorf("failed to create a new session (%v)", err)
	}
	return iam.New(sess), nil
}

// createIAMResources creates IAM resources needed to run CSI tests. If an error occurs, any created IAm resources are
// automatically deleted and the returned *iamResources is nil.
// awsRegion must be a valid AWS region for ec2 instances. For a complete list, see entries under "Region" on the table
// "Amazon Elastic Compute Cloud (Amazon EC2)": https://docs.aws.amazon.com/general/latest/gr/rande.html#ec2_region
func createIAMResources(awsRegion string) (*iamResources, error) {
	resources := &iamResources{}
	var err error

	// Creates new logger
	lg, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger (%v)", err)
	}
	resources.lg = lg

	// Creates new session and IAM client
	resources.svc, err = createIAM(awsRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to create session and IAM client (%v)", err)
	}
	resources.lg.Info("created session and IAM client")

	defer func() {
		// Delay is needed to ensure that permissions have been propagated.
		// See the section "Launching an Instance with an IAM Role" at
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
		time.Sleep(10 * time.Second)

		if err != nil {
			if deleteErr := resources.deleteIAMResources(); deleteErr != nil {
				resources.lg.Error("failed to delete all IAM resources", zap.Error(deleteErr))
			}
		}
	}()

	now := time.Now().UTC()
	uniqueSuffix := fmt.Sprintf("%x%x%x%x%x", now.Year(), int(now.Month()), now.Day(), now.Minute(), now.Second())

	// Creates instance profile
	instanceOutput, err := resources.svc.CreateInstanceProfile(&iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(fmt.Sprintf("a8t-csi-instance-profile-%s", uniqueSuffix)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add role to create new instance profile (%v)", err)
	}
	resources.instanceProfile = &iamResource{
		name: *instanceOutput.InstanceProfile.InstanceProfileName,
		arn:  *instanceOutput.InstanceProfile.Arn,
	}
	resources.lg.Info("created instance profile", zap.String("name", resources.instanceProfile.name))

	// Creates policy
	policyOutput, err := resources.svc.CreatePolicy(&iam.CreatePolicyInput{
		Description:    aws.String("awe-k8s-tester generated policy for testing EC2"),
		PolicyDocument: aws.String(policyDocument),
		PolicyName:     aws.String(fmt.Sprintf("a8t-csi-policy-%s", uniqueSuffix)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add role to create new policy(%v)", err)
	}
	resources.policy = &iamResource{
		name: *policyOutput.Policy.PolicyName,
		arn:  *policyOutput.Policy.Arn,
	}
	resources.lg.Info("created policy", zap.String("name", resources.policy.name))

	// Creates role
	roleOutput, err := resources.svc.CreateRole(&iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(assumeRoleDocument),
		RoleName:                 aws.String(fmt.Sprintf("a8t-csi-role-%s", uniqueSuffix)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new role (%v)", err)
	}
	resources.role = &iamResource{
		name: *roleOutput.Role.RoleName,
		arn:  *roleOutput.Role.Arn,
	}
	resources.lg.Info("created role", zap.String("name", resources.role.name))

	// Attaches role to policy
	_, err = resources.svc.AttachRolePolicy(&iam.AttachRolePolicyInput{
		PolicyArn: &resources.policy.arn,
		RoleName:  &resources.role.name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach role to policy (%v)", err)
	}
	resources.lg.Info("attached role to policy")

	// Adds role to instance profile
	_, err = resources.svc.AddRoleToInstanceProfile(&iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: &resources.instanceProfile.name,
		RoleName:            &resources.role.name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add role to instance profile (%v)", err)
	}
	resources.lg.Info("attached role to instance policy")

	resources.lg.Info("successfully created all IAM resources")
	return resources, nil
}

// Deletes instance profile, policy, and role from provided resources.
func (resources *iamResources) deleteIAMResources() error {
	errors := []string{}
	// Removes role from instance profile and deletes instance profile.
	// AWS does not allow instance profile to be deleted with role still attached.
	// https://docs.aws.amazon.com/IAM/latest/APIReference/API_DeleteInstanceProfile.html
	if resources.instanceProfile != nil {
		// Removes role from instance profile.
		_, err := resources.svc.RemoveRoleFromInstanceProfile(&iam.RemoveRoleFromInstanceProfileInput{
			InstanceProfileName: &resources.instanceProfile.name,
			RoleName:            &resources.role.name,
		})
		if err != nil {
			errors = append(errors,
				fmt.Sprintf("failed to remove role %q from instance profile %q (%v)",
					resources.role.name,
					resources.instanceProfile.name, err,
				),
			)
		} else {
			resources.lg.Info("removed role from instance profile",
				zap.String("role-name", resources.role.name),
				zap.String("instance-profile-name", resources.instanceProfile.name),
			)
			// Deletes instance profile.
			_, err := resources.svc.DeleteInstanceProfile(&iam.DeleteInstanceProfileInput{
				InstanceProfileName: &resources.instanceProfile.name,
			})
			if err != nil {
				errors = append(errors,
					fmt.Sprintf("failed to delete instance profile %q (%v)",
						resources.instanceProfile.name, err))
			} else {
				resources.lg.Info("deleted instance profile")
				resources.instanceProfile = nil
			}
		}
	}

	// Detaches policy from role and deletes policy.
	if resources.policy != nil {
		// Detaches policy from role.
		// AWS does not allow policy to be deleted with role still attached.
		_, err := resources.svc.DetachRolePolicy(&iam.DetachRolePolicyInput{
			PolicyArn: &resources.policy.arn,
			RoleName:  &resources.role.name,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to detach policy %q from role %q (%v)",
				resources.policy.name, resources.role.name, err))
		} else {
			resources.lg.Info("detached policy from role",
				zap.String("policy-name", resources.policy.name),
				zap.String("role-name", resources.role.name),
			)
			// Deletes policy.
			_, err := resources.svc.DeletePolicy(&iam.DeletePolicyInput{
				PolicyArn: &resources.policy.arn,
			})
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete policy %q (%v)",
					resources.policy.name, err))
			} else {
				resources.lg.Info("deleted policy")
				resources.policy = nil
			}
		}
	}

	// Deletes role.
	if resources.role != nil {
		_, err := resources.svc.DeleteRole(&iam.DeleteRoleInput{
			RoleName: &resources.role.name,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete role %q (%v)", resources.role.name, err))
		} else {
			resources.lg.Info("deleted role")
			resources.role = nil
		}
	}

	if len(errors) != 0 {
		return fmt.Errorf(strings.Join(errors, ", "))
	}

	resources.lg.Info("successfully deleted all IAM resources")
	return nil
}

func (resources *iamResources) getManualDeleteCommands() string {
	deleteCommands := []string{}

	if resources.instanceProfile != nil {
		if resources.role != nil {
			deleteCommands = append(deleteCommands,
				fmt.Sprintf("aws iam remove-role-from-instance-profile --instance-profile-name %s --role-name %s",
					resources.instanceProfile.name,
					resources.role.name,
				),
			)
		}
		deleteCommands = append(deleteCommands,
			fmt.Sprintf("aws iam delete-instance-profile --instance-profile-name %s", resources.instanceProfile.name))
	}

	if resources.role != nil {
		if resources.policy != nil {
			deleteCommands = append(deleteCommands,
				fmt.Sprintf("aws iam detach-role-policy --role-name %s --policy-arn %s",
					resources.role.name,
					resources.policy.arn,
				),
			)
		}
		deleteCommands = append(deleteCommands, fmt.Sprintf("aws iam delete-role --role-name %s", resources.role.name))
	}

	if resources.policy != nil {
		deleteCommands = append(deleteCommands, fmt.Sprintf("aws iam delete-policy --policy-arn %s", resources.policy.arn))
	}

	return strings.Join(deleteCommands, " \\\n  ")
}
