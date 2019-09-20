package eks

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) createVPC() error {
	if md.cfg.CFStackVPCName == "" {
		return errors.New("cannot create empty VPC stack")
	}

	now := time.Now().UTC()
	h, _ := os.Hostname()
	v := vpcStack{
		Description:       md.cfg.ClusterName + "-vpc-stack",
		Tag:               md.cfg.Tag,
		TagValue:          md.cfg.ClusterName,
		Hostname:          h,
		SecurityGroupName: md.cfg.ClusterName + "-security-group",
	}
	s, err := createVPCTemplate(v)
	if err != nil {
		return err
	}

	cfnInput := &cloudformation.CreateStackInput{
		StackName: aws.String(md.cfg.CFStackVPCName),
		Tags: []*cloudformation.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(md.cfg.ClusterName),
			},
			{
				Key:   aws.String("HOSTNAME"),
				Value: aws.String(h),
			},
		},

		// TemplateURL: aws.String("https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-vpc-sample.yaml"),
		TemplateBody: aws.String(s),
	}
	if md.cfg.CFStackVPCParameterVPCBlock != "" &&
		md.cfg.CFStackVPCParameterSubnet01Block != "" &&
		md.cfg.CFStackVPCParameterSubnet02Block != "" &&
		md.cfg.CFStackVPCParameterSubnet03Block != "" {
		cfnInput.Parameters = []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("VpcBlock"),
				ParameterValue: aws.String(md.cfg.CFStackVPCParameterVPCBlock),
			},
			{
				ParameterKey:   aws.String("Subnet01Block"),
				ParameterValue: aws.String(md.cfg.CFStackVPCParameterSubnet01Block),
			},
			{
				ParameterKey:   aws.String("Subnet02Block"),
				ParameterValue: aws.String(md.cfg.CFStackVPCParameterSubnet02Block),
			},
			{
				ParameterKey:   aws.String("Subnet03Block"),
				ParameterValue: aws.String(md.cfg.CFStackVPCParameterSubnet03Block),
			},
		}
	}
	_, err = md.cfn.CreateStack(cfnInput)
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusVPCCreated = true
	md.cfg.Sync()

	// usually take 1-minute
	md.lg.Info("waiting for 1-minute")
	select {
	case <-md.stopc:
		md.lg.Info("interrupted VPC stack creation")
		return nil
	case <-time.After(time.Minute):
	}

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		var do *cloudformation.DescribeStacksOutput
		do, err = md.cfn.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(md.cfg.CFStackVPCName),
		})
		if err != nil {
			md.lg.Warn("failed to describe VPC stack",
				zap.String("stack-name", md.cfg.CFStackVPCName),
				zap.Error(err),
			)
			md.cfg.CFStackVPCStatus = err.Error()
			time.Sleep(10 * time.Second)
			continue
		}

		if len(do.Stacks) != 1 {
			return fmt.Errorf("%q expects 1 Stack, got %v", md.cfg.CFStackVPCName, do.Stacks)
		}

		md.cfg.CFStackVPCStatus = *do.Stacks[0].StackStatus
		if isCFCreateFailed(md.cfg.CFStackVPCStatus) {
			return fmt.Errorf("failed to create %q (%q)", md.cfg.CFStackVPCName, md.cfg.CFStackVPCStatus)
		}

		for _, op := range do.Stacks[0].Outputs {
			if *op.OutputKey == "VpcId" {
				md.cfg.VPCID = *op.OutputValue
				continue
			}
			if *op.OutputKey == "SubnetIds" {
				vv := *op.OutputValue
				md.cfg.SubnetIDs = strings.Split(vv, ",")
				continue
			}
			if *op.OutputKey == "SecurityGroups" {
				md.cfg.SecurityGroupID = *op.OutputValue
			}
		}

		if md.cfg.CFStackVPCStatus == "CREATE_COMPLETE" {
			_, err = md.ec2.CreateTags(&ec2.CreateTagsInput{
				Resources: aws.StringSlice([]string{md.cfg.SecurityGroupID}),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(md.cfg.ClusterName),
					},
				},
			})
			if err != nil {
				md.lg.Warn("failed to tag security group", zap.String("group-id", md.cfg.SecurityGroupID), zap.Error(err))
			}
			break
		}

		md.lg.Info("creating VPC stack",
			zap.String("stack-name", md.cfg.CFStackVPCName),
			zap.String("stack-status", md.cfg.CFStackVPCStatus),
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
		)

		time.Sleep(10 * time.Second)
	}
	if err != nil {
		md.lg.Info("failed to create VPC stack",
			zap.String("stack-name", md.cfg.CFStackVPCName),
			zap.String("stack-status", md.cfg.CFStackVPCStatus),
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	md.lg.Info("created VPC stack",
		zap.String("stack-name", md.cfg.CFStackVPCName),
		zap.String("stack-status", md.cfg.CFStackVPCStatus),
		zap.String("vpc-id", md.cfg.VPCID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteVPC() error {
	if !md.cfg.ClusterState.StatusVPCCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusVPCCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.CFStackVPCName == "" {
		return errors.New("cannot delete empty VPC stack")
	}

	_, err := md.cfn.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(md.cfg.CFStackVPCName),
	})
	if err != nil {
		md.cfg.CFStackVPCStatus = err.Error()
		return err
	}

	// usually take 1-minute
	md.lg.Info("waiting for 1-minute")
	time.Sleep(time.Minute)

	now := time.Now().UTC()
	for time.Now().UTC().Sub(now) < 5*time.Minute {
		var do *cloudformation.DescribeStacksOutput
		do, err = md.cfn.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(md.cfg.CFStackVPCName),
		})
		if err == nil {
			md.cfg.CFStackVPCStatus = *do.Stacks[0].StackStatus
			md.lg.Info("deleting VPC stack",
				zap.String("stack-status", md.cfg.CFStackVPCStatus),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			time.Sleep(10 * time.Second)

			if time.Now().UTC().Sub(now) > 3*time.Minute {
				// TODO: this doesn't work because of dependencies...
				// e.g. DependencyViolation: The vpc 'vpc-0127f6d18bd98836a' has dependencies and cannot be deleted
				// had to manually delete from console to delete VPN connection
				cs, cerr := md.ec2.DescribeVpnConnections(&ec2.DescribeVpnConnectionsInput{})
				if cerr == nil {
					ids := make([]string, 0, len(cs.VpnConnections))
					for _, cv := range cs.VpnConnections {
						ids = append(ids, *cv.VpnConnectionId)
					}
					md.lg.Info("current VPC connections", zap.Int("number", len(ids)))
				}
				_, verr := md.ec2.DeleteVpc(&ec2.DeleteVpcInput{
					VpcId: aws.String(md.cfg.VPCID),
				})
				md.lg.Info(
					"manually tried to delete VPC",
					zap.String("vpc-id", md.cfg.VPCID),
					zap.Error(verr),
				)
				if verr != nil && strings.Contains(verr.Error(), "does not exist") {
					err = nil
					md.cfg.CFStackVPCStatus = "DELETE_COMPLETE"
					break
				}
			}
			continue
		}

		if isCFDeletedGoClient(md.cfg.CFStackVPCName, err) {
			err = nil
			md.cfg.CFStackVPCStatus = "DELETE_COMPLETE"
			break
		}
		md.cfg.CFStackVPCStatus = err.Error()

		md.lg.Warn("failed to describe VPC stack", zap.String("stack-name", md.cfg.CFStackVPCName), zap.Error(err))
		time.Sleep(10 * time.Second)
	}

	if err != nil {
		md.lg.Info("failed to delete VPC stack",
			zap.String("stack-name", md.cfg.CFStackVPCName),
			zap.String("stack-status", md.cfg.CFStackVPCStatus),
			zap.String("vpc-id", md.cfg.VPCID),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	md.lg.Info("deleted VPC stack",
		zap.String("stack-name", md.cfg.CFStackVPCName),
		zap.String("stack-status", md.cfg.CFStackVPCStatus),
		zap.String("vpc-id", md.cfg.VPCID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

// isCFCreateFailed return true if cloudformation status indicates its creation failure.
func isCFCreateFailed(status string) bool {
	/*
		https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html

		CREATE_IN_PROGRESS
		CREATE_FAILED
		CREATE_COMPLETE
		ROLLBACK_IN_PROGRESS
		ROLLBACK_FAILED
		ROLLBACK_COMPLETE
		DELETE_IN_PROGRESS
		DELETE_FAILED
		DELETE_COMPLETE
		UPDATE_IN_PROGRESS
		UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
		UPDATE_COMPLETE
		UPDATE_ROLLBACK_IN_PROGRESS
		UPDATE_ROLLBACK_FAILED
		UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
		UPDATE_ROLLBACK_COMPLETE
		REVIEW_IN_PROGRESS
	*/
	if strings.HasPrefix(status, "REVIEW_") || strings.HasPrefix(status, "CREATE_") {
		return false
	}
	return true
}

// isCFDeletedGoClient returns true if cloudformation errror indicates
// that the stack has already been deleted.
func isCFDeletedGoClient(clusterName string, err error) bool {
	if err == nil {
		return false
	}
	// ValidationError: Stack with id AWSTESTER-155460CAAC98A17003-CF-STACK-VPC does not exist\n\tstatus code: 400, request id: bf45410b-b863-11e8-9550-914acc220b7c
	notExistErr := fmt.Sprintf(`ValidationError: Stack with id %s does not exist`, clusterName)
	return strings.Contains(err.Error(), notExistErr)
}
