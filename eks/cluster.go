package eks

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	awsapicfn "github.com/aws/aws-k8s-tester/pkg/awsapi/cloudformation"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	awseks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// MAKE SURE TO SYNC THE DEFAULT VALUES in "eksconfig"

// TemplateCluster is the CloudFormation template for EKS cluster.
const TemplateCluster = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster'

Parameters:

  ClusterName:
    Description: Cluster name
    Type: String

  Version:
    Description: Specify the EKS version
    Type: String
    Default: 1.14
    AllowedValues:
    - 1.14

  ClusterRoleARN:
    Description: Role ARN that EKS uses to create AWS resources for Kubernetes
    Type: String

  PrivateSubnetIDs:
    Description: All private subnets in the VPC
    Type: CommaDelimitedList

  ControlPlaneSecurityGroupID:
    Description: Security group ID for the cluster control plane communication with worker nodes
    Type: AWS::EC2::SecurityGroup::Id

Resources:

  Cluster:
    Type: AWS::EKS::Cluster
    Properties:
      Name: !Ref ClusterName
      Version: !Ref Version
      RoleArn: !Ref ClusterRoleARN
      ResourcesVpcConfig:
        SubnetIds: !Ref PrivateSubnetIDs
        SecurityGroupIds:
        - !Ref ControlPlaneSecurityGroupID

Outputs:

  ClusterARN:
    Description: EKS Cluster ARN
    Value: !GetAtt Cluster.Arn

  ClusterAPIServerEndpoint:
    Description: EKS Cluster API server endpoint
    Value: !GetAtt Cluster.Endpoint

`

func (ts *Tester) createCluster() error {
	if err := ts.createEKS(); err != nil {
		return err
	}
	if err := ts.updateKUBECONFIG(); err != nil {
		return err
	}
	if err := ts.pollClusterInfo(3*time.Minute, 10*time.Second); err != nil {
		return err
	}
	if err := ts.updateK8sClientSet(); err != nil {
		return err
	}
	return ts.cfg.Sync()
}

func (ts *Tester) createEKS() error {
	if ts.cfg.Status.ClusterCFNStackID != "" ||
		ts.cfg.Status.ClusterARN != "" ||
		ts.cfg.Status.ClusterAPIServerEndpoint != "" ||
		ts.cfg.Status.ClusterCA != "" ||
		ts.cfg.Status.ClusterCADecoded != "" ||
		ts.cfg.Status.ClusterStatus != "" {
		ts.lg.Info("non-empty cluster given; no need to create a new one")
		return nil
	}
	if ts.cfg.Status.Up {
		ts.lg.Info("cluster is up; no need to create cluster")
		return nil
	}
	ts.describeCluster()
	if ts.cfg.Status.ClusterStatus == ClusterStatusACTIVE {
		return fmt.Errorf("%q already %q", ts.cfg.Name, ts.cfg.Status.ClusterStatus)
	}

	now := time.Now().UTC()
	initialWait := 7 * time.Minute

	if ts.cfg.Parameters.ClusterResolverURL != "" || (ts.cfg.Parameters.ClusterRequestHeaderKey != "" && ts.cfg.Parameters.ClusterRequestHeaderValue != "") {
		ts.lg.Info("creating a cluster using EKS API",
			zap.String("name", ts.cfg.Name),
			zap.String("resolver-url", ts.cfg.Parameters.ClusterResolverURL),
			zap.String("signing-name", ts.cfg.Parameters.ClusterSigningName),
			zap.String("request-header-key", ts.cfg.Parameters.ClusterRequestHeaderKey),
			zap.String("request-header-value", ts.cfg.Parameters.ClusterRequestHeaderValue),
		)
		createInput := awseks.CreateClusterInput{
			Name:    aws.String(ts.cfg.Name),
			Version: aws.String(ts.cfg.Parameters.Version),
			RoleArn: aws.String(ts.cfg.Status.ClusterRoleARN),
			ResourcesVpcConfig: &awseks.VpcConfigRequest{
				SubnetIds:        aws.StringSlice(ts.cfg.Status.PrivateSubnetIDs),
				SecurityGroupIds: aws.StringSlice([]string{ts.cfg.Status.ControlPlaneSecurityGroupID}),
			},
			Tags: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
			},
		}
		for k, v := range ts.cfg.Parameters.ClusterTags {
			createInput.Tags[k] = aws.String(v)
			ts.lg.Info("added EKS tag", zap.String("key", k), zap.String("value", v))
		}
		req, _ := ts.eksAPI.CreateClusterRequest(&createInput)
		if ts.cfg.Parameters.ClusterRequestHeaderKey != "" && ts.cfg.Parameters.ClusterRequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.Parameters.ClusterRequestHeaderKey] = []string{ts.cfg.Parameters.ClusterRequestHeaderValue}
			ts.lg.Info("set request header for EKS create request",
				zap.String("key", ts.cfg.Parameters.ClusterRequestHeaderKey),
				zap.String("value", ts.cfg.Parameters.ClusterRequestHeaderValue),
			)
		}
		err := req.Send()
		if err != nil {
			return err
		}
		ts.lg.Info("sent create cluster request")

	} else {

		initialWait = time.Minute
		ts.lg.Info("creating a cluster using CFN", zap.String("name", ts.cfg.Name))
		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(ts.cfg.Name + "-cluster"),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
			OnFailure:    aws.String("DELETE"),
			TemplateBody: aws.String(TemplateCluster),
			Tags: awsapicfn.NewTags(map[string]string{
				"Kind": "aws-k8s-tester",
				"Name": ts.cfg.Name,
			}),
			Parameters: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String("ClusterName"),
					ParameterValue: aws.String(ts.cfg.Name),
				},
				{
					ParameterKey:   aws.String("Version"),
					ParameterValue: aws.String(ts.cfg.Parameters.Version),
				},
				{
					ParameterKey:   aws.String("ClusterRoleARN"),
					ParameterValue: aws.String(ts.cfg.Status.ClusterRoleARN),
				},
				{
					ParameterKey:   aws.String("PrivateSubnetIDs"),
					ParameterValue: aws.String(strings.Join(ts.cfg.Status.PrivateSubnetIDs, ",")),
				},
				{
					ParameterKey:   aws.String("ControlPlaneSecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.Status.ControlPlaneSecurityGroupID),
				},
			},
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		ts.cfg.Status.ClusterCFNStackID = aws.StringValue(stackOutput.StackId)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awsapicfn.Poll(
			ctx,
			ts.stopCreationCh,
			ts.interruptSig,
			ts.lg,
			ts.cfnAPI,
			ts.cfg.Status.ClusterCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			7*time.Minute,
			30*time.Second,
		)
		var st awsapicfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to create cluster (%v)", st.Error)
				ts.cfg.Sync()
				ts.lg.Error("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		// update status after creating a new cluster
		for _, o := range st.Stack.Outputs {
			switch k := aws.StringValue(o.OutputKey); k {
			case "ClusterARN":
				ts.cfg.Status.ClusterARN = aws.StringValue(o.OutputValue)
			case "ClusterAPIServerEndpoint":
				ts.cfg.Status.ClusterAPIServerEndpoint = aws.StringValue(o.OutputValue)
			default:
				return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.Status.ClusterCFNStackID)
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	ch := PollEKS(
		ctx,
		ts.stopCreationCh,
		ts.lg,
		ts.eksAPI,
		ts.cfg.Name,
		ClusterStatusACTIVE,
		initialWait,
		30*time.Second,
	)
	for v := range ch {
		ts.updateClusterStatus(v)
	}
	cancel()

	ts.lg.Info("created a cluster",
		zap.String("cluster-cfn-stack-id", ts.cfg.Status.ClusterCFNStackID),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
		zap.String("cluster-api-server-endpoint", ts.cfg.Status.ClusterAPIServerEndpoint),
		zap.Int("cluster-ca-bytes", len(ts.cfg.Status.ClusterCA)),
		zap.String("config-path", ts.cfg.ConfigPath),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return ts.cfg.Sync()
}

// https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status
//
//  CREATING
//  ACTIVE
//  UPDATING
//  DELETING
//  FAILED
//
const (
	ClusterStatusCREATING          = "CREATING"
	ClusterStatusACTIVE            = "ACTIVE"
	ClusterStatusUPDATING          = "UPDATING"
	ClusterStatusDELETING          = "DELETING"
	ClusterStatusFAILED            = "FAILED"
	ClusterStatusDELETEDORNOTEXIST = "DELETED/NOT-EXIST"
)

func (ts *Tester) describeCluster() {
	dout, err := ts.eksAPI.DescribeCluster(&awseks.DescribeClusterInput{Name: aws.String(ts.cfg.Name)})
	if err != nil {
		if ClusterDeleted(err) {
			ts.cfg.Status.ClusterStatus = ClusterStatusDELETEDORNOTEXIST
			ts.cfg.Status.Up = false
		} else {
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to describe cluster (%v)", err)
		}
	}
	if dout.Cluster == nil {
		ts.cfg.Status.ClusterStatus = ClusterStatusDELETEDORNOTEXIST
		ts.cfg.Status.Up = false
	} else {
		ts.cfg.Status.ClusterStatus = aws.StringValue(dout.Cluster.Status)
		ts.cfg.Status.Up = ts.cfg.Status.ClusterStatus == ClusterStatusACTIVE
	}
	ts.lg.Info("described cluster",
		zap.String("name", ts.cfg.Name),
		zap.String("cluster-status", ts.cfg.Status.ClusterStatus),
	)
}

func (ts *Tester) updateClusterStatus(v ClusterStatus) {
	if v.Cluster == nil {
		if v.Error != nil {
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed with error %v", v.Error)
		} else {
			ts.cfg.Status.ClusterStatus = ClusterStatusDELETEDORNOTEXIST
		}
		ts.cfg.Status.Up = false
		return
	}
	ts.cfg.Status.ClusterARN = aws.StringValue(v.Cluster.Arn)
	ts.cfg.Status.ClusterStatus = aws.StringValue(v.Cluster.Status)
	ts.cfg.Status.Up = ts.cfg.Status.ClusterStatus == ClusterStatusACTIVE
	if ts.cfg.Status.ClusterStatus != ClusterStatusDELETEDORNOTEXIST {
		if v.Cluster.Endpoint != nil {
			ts.cfg.Status.ClusterAPIServerEndpoint = aws.StringValue(v.Cluster.Endpoint)
		}
		if v.Cluster.Identity != nil && v.Cluster.Identity.Oidc != nil && v.Cluster.Identity.Oidc.Issuer != nil {
			ts.cfg.Status.ClusterOIDCIssuer = aws.StringValue(v.Cluster.Identity.Oidc.Issuer)
		}
		if v.Cluster.CertificateAuthority != nil && v.Cluster.CertificateAuthority.Data != nil {
			ts.cfg.Status.ClusterCA = aws.StringValue(v.Cluster.CertificateAuthority.Data)
		}
		d, err := base64.StdEncoding.DecodeString(ts.cfg.Status.ClusterCA)
		if err != nil {
			ts.lg.Error("failed to decode cluster CA", zap.Error(err))
		}
		ts.cfg.Status.ClusterCADecoded = string(d)
	} else {
		ts.cfg.Status.ClusterAPIServerEndpoint = ""
		ts.cfg.Status.ClusterOIDCIssuer = ""
		ts.cfg.Status.ClusterCA = ""
		ts.cfg.Status.ClusterCADecoded = ""
	}
	ts.cfg.Sync()
}

// ClusterDeleted returns true if error from EKS API indicates that
// the EKS cluster has already been deleted.
func ClusterDeleted(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if ok && awsErr.Code() == "ResourceNotFoundException" &&
		strings.HasPrefix(awsErr.Message(), "No cluster found for") {
		return true
	}
	// ResourceNotFoundException: No cluster found for name: aws-k8s-tester-155468BC717E03B003\n\tstatus code: 404, request id: 1e3fe41c-b878-11e8-adca-b503e0ba731d
	return strings.Contains(err.Error(), "No cluster found for name: ")
}

// ClusterStatus represents the CloudFormation status.
type ClusterStatus struct {
	Cluster *awseks.Cluster
	Error   error
}

// PollEKS periodically fetches the cluster status
// until the cluster becomes the desired state.
func PollEKS(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	eksAPI eksiface.EKSAPI,
	clusterName string,
	desiredClusterStatus string,
	initialWait time.Duration,
	wait time.Duration,
) <-chan ClusterStatus {
	lg.Info("polling cluster",
		zap.String("cluster-name", clusterName),
		zap.String("desired-cluster-status", desiredClusterStatus),
	)

	now := time.Now().UTC()

	ch := make(chan ClusterStatus, 10)
	go func() {
		ticker := time.NewTicker(wait)
		defer ticker.Stop()

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted", zap.Error(ctx.Err()))
				ch <- ClusterStatus{Cluster: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped", zap.Error(ctx.Err()))
				ch <- ClusterStatus{Cluster: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-ticker.C:
			}

			output, err := eksAPI.DescribeCluster(&awseks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil {
				if ClusterDeleted(err) {
					if desiredClusterStatus == ClusterStatusDELETEDORNOTEXIST {
						lg.Info("cluster is already deleted as desired; exiting", zap.Error(err))
						ch <- ClusterStatus{Cluster: nil, Error: nil}
						close(ch)
						return
					}

					lg.Warn("cluster does not exist; aborting", zap.Error(ctx.Err()))
					ch <- ClusterStatus{Cluster: nil, Error: err}
					close(ch)
					return
				}

				lg.Error("describe cluster failed; retrying", zap.Error(err))
				ch <- ClusterStatus{Cluster: nil, Error: err}
				continue
			}

			if output.Cluster == nil {
				lg.Error("expected non-nil cluster; retrying")
				ch <- ClusterStatus{Cluster: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			cluster := output.Cluster
			status := aws.StringValue(cluster.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("cluster-status", status),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			ch <- ClusterStatus{Cluster: cluster, Error: nil}
			if status == desiredClusterStatus {
				lg.Info("became desired cluster status; exiting", zap.String("cluster-status", status))
				close(ch)
				return
			}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
				time.Sleep(initialWait)
				first = false
			}
		}

		lg.Warn("wait aborted", zap.Error(ctx.Err()))
		ch <- ClusterStatus{Cluster: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

func (ts *Tester) deleteCluster() error {
	if ts.cfg.Status.ClusterStatus == "" || ts.cfg.Status.ClusterStatus == ClusterStatusDELETEDORNOTEXIST {
		ts.lg.Info("cluster already deleted; no need to delete cluster")
		return nil
	}
	if !ts.cfg.Status.Up {
		ts.lg.Info("cluster is not up; no need to delete cluster")
		return nil
	}

	ts.lg.Info("deleting cluster", zap.String("cluster-name", ts.cfg.Name))
	if ts.cfg.Status.ClusterCFNStackID != "" {
		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(ts.cfg.Status.ClusterCFNStackID),
		})
		if err != nil {
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to delete cluster (%v)", err)
			ts.cfg.Sync()
			return err
		}
		ts.cfg.Status.Up = false
		ts.cfg.Sync()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awsapicfn.Poll(
			ctx,
			make(chan struct{}),  // do not exit on stop
			make(chan os.Signal), // do not exit on stop
			ts.lg,
			ts.cfnAPI,
			ts.cfg.Status.ClusterCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			3*time.Minute,
			20*time.Second,
		)
		var st awsapicfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to delete cluster (%v)", st.Error)
				ts.cfg.Sync()
				ts.lg.Error("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.Status.ClusterStatus = ClusterStatusDELETEDORNOTEXIST
		ts.cfg.Status.Up = false
		ts.cfg.Sync()

	} else {

		_, err := ts.eksAPI.DeleteCluster(&awseks.DeleteClusterInput{
			Name: aws.String(ts.cfg.Name),
		})
		if err != nil {
			ts.cfg.Status.ClusterStatus = fmt.Sprintf("failed to delete cluster (%v)", err)
			ts.cfg.Sync()
			return err
		}
		ts.cfg.Status.Up = false
		ts.cfg.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		csCh := PollEKS(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.lg,
			ts.eksAPI,
			ts.cfg.Name,
			ClusterStatusDELETEDORNOTEXIST,
			3*time.Minute,
			20*time.Second,
		)
		for v := range csCh {
			ts.updateClusterStatus(v)
		}
		cancel()
	}

	ts.lg.Info("deleted a cluster",
		zap.String("cluster-cfn-stack-id", ts.cfg.Status.ClusterCFNStackID),
		zap.String("cluster-name", ts.cfg.Name),
	)
	return ts.cfg.Sync()
}
