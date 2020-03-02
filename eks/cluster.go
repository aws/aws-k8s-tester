package eks

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
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

  RoleARN:
    Description: Role ARN that EKS uses to create AWS resources for Kubernetes
    Type: String

  SubnetIDs:
    Description: Subnets for EKS worker nodes. Amazon EKS creates cross-account elastic network interfaces in these subnets to allow communication between  worker nodes and the Kubernetes control plane
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
      RoleArn: !Ref RoleARN
      ResourcesVpcConfig:
        SubnetIds: !Ref SubnetIDs
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
	if err := ts.createK8sClientSet(); err != nil {
		return err
	}
	return ts.cfg.Sync()
}

func (ts *Tester) createEKS() error {
	createStart := time.Now()
	defer func() {
		ts.cfg.Status.CreateTook = time.Since(createStart)
		ts.cfg.Status.CreateTookString = ts.cfg.Status.CreateTook.String()
		ts.cfg.Sync()
	}()

	if ts.cfg.Status.ClusterCFNStackID != "" ||
		ts.cfg.Status.ClusterARN != "" ||
		ts.cfg.Status.ClusterAPIServerEndpoint != "" ||
		ts.cfg.Status.ClusterCA != "" ||
		ts.cfg.Status.ClusterCADecoded != "" ||
		ts.cfg.Status.ClusterStatusCurrent != "" {
		ts.lg.Info("non-empty cluster given; no need to create a new one")
		return nil
	}
	if ts.cfg.Status.Up {
		ts.lg.Info("cluster is up; no need to create cluster")
		return nil
	}

	ts.describeCluster()
	if ts.cfg.Status.ClusterStatusCurrent == awseks.ClusterStatusActive {
		ts.lg.Info("cluster status is active; no need to create cluster", zap.String("status", ts.cfg.Status.ClusterStatusCurrent))
		return nil
	}

	now := time.Now()
	initialWait := 7*time.Minute + 30*time.Second

	subnets := make([]string, len(ts.cfg.Parameters.PublicSubnetIDs))
	copy(subnets, ts.cfg.Parameters.PublicSubnetIDs)
	if len(ts.cfg.Parameters.PrivateSubnetIDs) > 0 {
		subnets = append(subnets, ts.cfg.Parameters.PrivateSubnetIDs...)
	}

	if ts.cfg.Parameters.ResolverURL != "" ||
		(ts.cfg.Parameters.RequestHeaderKey != "" &&
			ts.cfg.Parameters.RequestHeaderValue != "") ||
		ts.cfg.Parameters.EncryptionCMKARN != "" { // TODO

		ts.lg.Info("creating a cluster using EKS API",
			zap.String("name", ts.cfg.Name),
			zap.String("resolver-url", ts.cfg.Parameters.ResolverURL),
			zap.String("signing-name", ts.cfg.Parameters.SigningName),
			zap.String("request-header-key", ts.cfg.Parameters.RequestHeaderKey),
			zap.String("request-header-value", ts.cfg.Parameters.RequestHeaderValue),
		)
		createInput := awseks.CreateClusterInput{
			Name:    aws.String(ts.cfg.Name),
			Version: aws.String(ts.cfg.Parameters.Version),
			RoleArn: aws.String(ts.cfg.Parameters.RoleARN),
			ResourcesVpcConfig: &awseks.VpcConfigRequest{
				SubnetIds:        aws.StringSlice(subnets),
				SecurityGroupIds: aws.StringSlice([]string{ts.cfg.Parameters.ControlPlaneSecurityGroupID}),
			},
			Tags: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
			},
		}
		for k, v := range ts.cfg.Parameters.Tags {
			createInput.Tags[k] = aws.String(v)
			ts.lg.Info("added EKS tag to EKS API request",
				zap.String("key", k),
				zap.String("value", v),
			)
		}
		if ts.cfg.Parameters.EncryptionCMKARN != "" {
			// TODO
			ts.lg.Info("added encryption to EKS API request",
				zap.String("cmk-arn", ts.cfg.Parameters.EncryptionCMKARN),
			)
		}
		req, _ := ts.eksAPI.CreateClusterRequest(&createInput)
		if ts.cfg.Parameters.RequestHeaderKey != "" && ts.cfg.Parameters.RequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.Parameters.RequestHeaderKey] = []string{ts.cfg.Parameters.RequestHeaderValue}
			ts.lg.Info("set request header for EKS create request",
				zap.String("key", ts.cfg.Parameters.RequestHeaderKey),
				zap.String("value", ts.cfg.Parameters.RequestHeaderValue),
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
			OnFailure:    aws.String(cloudformation.OnFailureDelete),
			TemplateBody: aws.String(TemplateCluster),
			Tags: awscfn.NewTags(map[string]string{
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
					ParameterKey:   aws.String("RoleARN"),
					ParameterValue: aws.String(ts.cfg.Parameters.RoleARN),
				},
				{
					ParameterKey:   aws.String("SubnetIDs"),
					ParameterValue: aws.String(strings.Join(subnets, ",")),
				},
				{
					ParameterKey:   aws.String("ControlPlaneSecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.Parameters.ControlPlaneSecurityGroupID),
				},
			},
		}
		if ts.cfg.Parameters.EncryptionCMKARN != "" {
			// TODO
			ts.lg.Info("added encryption config to EKS CFN request",
				zap.String("cmk-arn", ts.cfg.Parameters.EncryptionCMKARN),
			)
		}
		stackOutput, err := ts.cfnAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		ts.cfg.Status.ClusterCFNStackID = aws.StringValue(stackOutput.StackId)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
			ctx,
			ts.stopCreationCh,
			ts.interruptSig,
			ts.lg,
			ts.cfnAPI,
			ts.cfg.Status.ClusterCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			7*time.Minute+30*time.Second,
			30*time.Second,
		)
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.RecordStatus(fmt.Sprintf("failed to create cluster (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
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
	ch := Poll(
		ctx,
		ts.stopCreationCh,
		ts.lg,
		ts.eksAPI,
		ts.cfg.Name,
		awseks.ClusterStatusActive,
		initialWait,
		30*time.Second,
	)
	for v := range ch {
		ts.updateClusterStatus(v, awseks.ClusterStatusActive)
	}
	cancel()

	ts.lg.Info("created a cluster",
		zap.String("cluster-cfn-stack-id", ts.cfg.Status.ClusterCFNStackID),
		zap.String("cluster-arn", ts.cfg.Status.ClusterARN),
		zap.String("cluster-api-server-endpoint", ts.cfg.Status.ClusterAPIServerEndpoint),
		zap.Int("cluster-ca-bytes", len(ts.cfg.Status.ClusterCA)),
		zap.String("config-path", ts.cfg.ConfigPath),
		zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteCluster() error {
	deleteStart := time.Now()
	defer func() {
		ts.cfg.Status.DeleteTook = time.Since(deleteStart)
		ts.cfg.Status.DeleteTookString = ts.cfg.Status.DeleteTook.String()
		ts.cfg.Sync()
	}()

	ts.describeCluster()
	if ts.cfg.Status.ClusterStatusCurrent == "" || ts.cfg.Status.ClusterStatusCurrent == eksconfig.ClusterStatusDELETEDORNOTEXIST {
		ts.lg.Info("cluster already deleted; no need to delete cluster")
		return nil
	}

	ts.lg.Info("deleting cluster", zap.String("cluster-name", ts.cfg.Name))
	if ts.cfg.Status.ClusterCFNStackID != "" {

		_, err := ts.cfnAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(ts.cfg.Status.ClusterCFNStackID),
		})
		if err != nil {
			ts.cfg.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", err))
			return err
		}
		ts.cfg.Status.Up = false
		ts.cfg.Sync()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := awscfn.Poll(
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
		var st awscfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", st.Error))
				ts.lg.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)

	} else {

		_, err := ts.eksAPI.DeleteCluster(&awseks.DeleteClusterInput{
			Name: aws.String(ts.cfg.Name),
		})
		if err != nil {
			ts.cfg.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", err))
			return err
		}
		ts.cfg.Status.Up = false
		ts.cfg.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		csCh := Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.lg,
			ts.eksAPI,
			ts.cfg.Name,
			eksconfig.ClusterStatusDELETEDORNOTEXIST,
			3*time.Minute,
			20*time.Second,
		)
		for v := range csCh {
			ts.updateClusterStatus(v, eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		cancel()
	}

	ts.lg.Info("deleted a cluster",
		zap.String("cluster-cfn-stack-id", ts.cfg.Status.ClusterCFNStackID),
		zap.String("cluster-name", ts.cfg.Name),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) describeCluster() {
	dout, err := ts.eksAPI.DescribeCluster(&awseks.DescribeClusterInput{Name: aws.String(ts.cfg.Name)})
	if err != nil {
		if ClusterDeleted(err) {
			ts.cfg.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		} else {
			ts.cfg.RecordStatus(fmt.Sprintf("failed to describe cluster (%v)", err))
		}
	}
	if dout.Cluster == nil {
		ts.cfg.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
	} else {
		ts.cfg.RecordStatus(aws.StringValue(dout.Cluster.Status))
	}
	ts.lg.Info("described cluster",
		zap.String("name", ts.cfg.Name),
		zap.String("cluster-status", ts.cfg.Status.ClusterStatusCurrent),
	)
}

func (ts *Tester) updateClusterStatus(v ClusterStatus, desired string) {
	if v.Cluster == nil {
		if v.Error != nil {
			ts.cfg.RecordStatus(fmt.Sprintf("failed with error %v", v.Error))
			ts.cfg.Status.Up = false
		} else {
			ts.cfg.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		return
	}
	ts.cfg.Status.ClusterARN = aws.StringValue(v.Cluster.Arn)
	ts.cfg.RecordStatus(aws.StringValue(v.Cluster.Status))

	if desired != eksconfig.ClusterStatusDELETEDORNOTEXIST && ts.cfg.Status.ClusterStatusCurrent != eksconfig.ClusterStatusDELETEDORNOTEXIST {

		if v.Cluster.Endpoint != nil {
			ts.cfg.Status.ClusterAPIServerEndpoint = aws.StringValue(v.Cluster.Endpoint)
		}

		if v.Cluster.Identity != nil &&
			v.Cluster.Identity.Oidc != nil &&
			v.Cluster.Identity.Oidc.Issuer != nil {
			ts.cfg.Status.ClusterOIDCIssuerURL = aws.StringValue(v.Cluster.Identity.Oidc.Issuer)
			u, err := url.Parse(ts.cfg.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.lg.Warn(
					"failed to parse ClusterOIDCIssuerURL",
					zap.String("url", ts.cfg.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			if u.Scheme != "https" {
				ts.lg.Warn("invalid scheme", zap.String("scheme", u.Scheme))
			}
			if u.Port() == "" {
				ts.lg.Info("updating host with port :443", zap.String("host", u.Host))
				u.Host += ":443"
			}
			ts.cfg.Status.ClusterOIDCIssuerURL = u.String()
			ts.cfg.Status.ClusterOIDCIssuerHostPath = u.Hostname() + u.Path
			ts.cfg.Status.ClusterOIDCIssuerARN = fmt.Sprintf(
				"arn:aws:iam::%s:oidc-provider/%s",
				ts.cfg.Status.AWSAccountID,
				ts.cfg.Status.ClusterOIDCIssuerHostPath,
			)

			ts.lg.Info("fetching OIDC CA thumbprint", zap.String("url", ts.cfg.Status.ClusterOIDCIssuerURL))
			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{},
					Proxy:           http.ProxyFromEnvironment,
				},
			}
			resp, err := httpClient.Get(ts.cfg.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.lg.Warn("failed to fetch OIDC CA thumbprint",
					zap.String("url", ts.cfg.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			defer resp.Body.Close()

			if resp.TLS != nil {
				certs := len(resp.TLS.PeerCertificates)
				if certs >= 1 {
					root := resp.TLS.PeerCertificates[certs-1]
					ts.cfg.Status.ClusterOIDCIssuerCAThumbprint = fmt.Sprintf("%x", sha1.Sum(root.Raw))
					ts.lg.Info("fetched OIDC CA thumbprint")
				} else {
					ts.lg.Warn("received empty TLS peer certs")
				}
			} else {
				ts.lg.Warn("received empty HTTP empty TLS response")
			}
		}

		if v.Cluster.CertificateAuthority != nil && v.Cluster.CertificateAuthority.Data != nil {
			ts.cfg.Status.ClusterCA = aws.StringValue(v.Cluster.CertificateAuthority.Data)
		}
		d, err := base64.StdEncoding.DecodeString(ts.cfg.Status.ClusterCA)
		if err != nil {
			ts.lg.Warn("failed to decode cluster CA", zap.Error(err))
		}
		ts.cfg.Status.ClusterCADecoded = string(d)

	} else {

		ts.cfg.Status.ClusterAPIServerEndpoint = ""
		ts.cfg.Status.ClusterOIDCIssuerURL = ""
		ts.cfg.Status.ClusterOIDCIssuerHostPath = ""
		ts.cfg.Status.ClusterOIDCIssuerARN = ""
		ts.cfg.Status.ClusterOIDCIssuerCAThumbprint = ""
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

// Poll periodically fetches the cluster status
// until the cluster becomes the desired state.
func Poll(
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

	now := time.Now()

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
					if desiredClusterStatus == eksconfig.ClusterStatusDELETEDORNOTEXIST {
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

				lg.Warn("describe cluster failed; retrying", zap.Error(err))
				ch <- ClusterStatus{Cluster: nil, Error: err}
				continue
			}

			if output.Cluster == nil {
				lg.Warn("expected non-nil cluster; retrying")
				ch <- ClusterStatus{Cluster: nil, Error: fmt.Errorf("unexpected empty response %+v", output.GoString())}
				continue
			}

			cluster := output.Cluster
			currentStatus := aws.StringValue(cluster.Status)
			lg.Info("poll",
				zap.String("cluster-name", clusterName),
				zap.String("cluster-status", currentStatus),
				zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredClusterStatus:
				ch <- ClusterStatus{Cluster: cluster, Error: nil}
				lg.Info("became desired cluster status; exiting", zap.String("cluster-status", currentStatus))
				close(ch)
				return
			case awseks.ClusterStatusFailed:
				ch <- ClusterStatus{Cluster: cluster, Error: fmt.Errorf("unexpected cluster status %q", awseks.ClusterStatusFailed)}
				close(ch)
				return
			default:
				ch <- ClusterStatus{Cluster: cluster, Error: nil}
			}
			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))
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
				case <-time.After(initialWait):
				}
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
