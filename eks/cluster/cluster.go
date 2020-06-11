// Package cluster implements EKS cluster tester.
package cluster

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks"
	aws_eks "github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"github.com/dustin/go-humanize"
	"github.com/mitchellh/colorstring"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	IAMAPI    iamiface.IAMAPI
	KMSAPI    kmsiface.KMSAPI
	CFNAPI    cloudformationiface.CloudFormationAPI
	EC2API    ec2iface.EC2API
	EKSAPI    eksiface.EKSAPI
	ELBV2API  elbv2iface.ELBV2API
}

type Tester interface {
	// Create creates EKS cluster, and waits for completion.
	Create() error
	Client() k8s_client.EKS
	// CheckHealth checks EKS cluster health.
	CheckHealth() error
	// Delete deletes all EKS cluster resources.
	Delete() error
}

// New creates a new Job tester.
func New(cfg Config) Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg, checkHealthMu: new(sync.Mutex)}
}

type tester struct {
	cfg       Config
	k8sClient k8s_client.EKS

	checkHealthMu *sync.Mutex
}

func (ts *tester) Create() (err error) {
	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))

	if err = ts.createEncryption(); err != nil {
		return err
	}
	if err = ts.createClusterRole(); err != nil {
		return err
	}
	if err = ts.createVPC(); err != nil {
		return err
	}
	if err = ts.createEKS(); err != nil {
		return err
	}
	ts.k8sClient, err = ts.createClient()
	if err != nil {
		return err
	}
	if err = ts.CheckHealth(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Client() k8s_client.EKS { return ts.k8sClient }

func (ts *tester) CheckHealth() (err error) {
	ts.checkHealthMu.Lock()
	defer ts.checkHealthMu.Unlock()
	return ts.checkHealth()
}

func (ts *tester) checkHealth() (err error) {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************[default]\n")
		colorstring.Printf("[light_green]checkHealth [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("checkHealth (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	defer func() {
		if err == nil {
			ts.cfg.EKSConfig.RecordStatus(eks.ClusterStatusActive)
		}
	}()

	if ts.k8sClient == nil {
		ts.cfg.Logger.Info("empty client; creating client")
		ts.k8sClient, err = ts.createClient()
		if err != nil {
			return err
		}
	}

	// might take several minutes for DNS to propagate
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("health check aborted")
		case <-time.After(5 * time.Second):
		}
		if ts.cfg.EKSConfig.Status == nil {
			ts.cfg.Logger.Warn("empty EKSConfig.Status")
		} else {
			ts.cfg.EKSConfig.Status.ServerVersionInfo, err = ts.k8sClient.FetchServerVersion()
			if err != nil {
				ts.cfg.Logger.Warn("get version failed", zap.Error(err))
			}
		}
		err = ts.k8sClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("health check failed (%v)", err))
	}

	ts.cfg.Logger.Info("health check success")
	return err
}

func (ts *tester) Delete() error {
	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))

	var errs []string

	if err := ts.deleteEKS(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteEncryption(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteClusterRole(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteVPC(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return ts.cfg.EKSConfig.Sync()
}

// MAKE SURE TO SYNC THE DEFAULT VALUES in "eksconfig"

// TemplateCluster is the CloudFormation template for EKS cluster.
const TemplateCluster = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster'

Parameters:

  ClusterName:
    Type: String
    Description: Cluster name

  Version:
    Type: String
    Default: 1.16
    Description: Specify the EKS version

  RoleARN:
    Type: String
    Description: Role ARN that EKS uses to create AWS resources for Kubernetes

  SubnetIDs:
    Type: List<AWS::EC2::Subnet::Id>
    Description: Subnets for EKS worker nodes. Amazon EKS creates cross-account elastic network interfaces in these subnets to allow communication between  worker nodes and the Kubernetes control plane

  ClusterControlPlaneSecurityGroupID:
    Type: AWS::EC2::SecurityGroup::Id
    Description: Security group ID for the cluster control plane communication with worker nodes
{{ if ne .AWSEncryptionProviderCMKARN "" }}
  AWSEncryptionProviderCMKARN:
    Type: String
    Description: KMS CMK for aws-encryption-provider.
{{ end }}

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
        - !Ref ClusterControlPlaneSecurityGroupID
{{ if ne .AWSEncryptionProviderCMKARN "" }}      EncryptionConfig:
      - Resources:
        - secrets
        Provider:
          KeyArn: !Ref AWSEncryptionProviderCMKARN
{{ end }}
Outputs:

  ClusterARN:
    Value: !GetAtt Cluster.Arn
    Description: EKS Cluster ARN

  ClusterAPIServerEndpoint:
    Value: !GetAtt Cluster.Endpoint
    Description: EKS Cluster API server endpoint

`

type templateEKSCluster struct {
	AWSEncryptionProviderCMKARN string
}

func (ts *tester) createEKS() (err error) {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************[default]\n")
		colorstring.Printf("[light_green]createEKS [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("createEKS (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	if ts.cfg.EKSConfig.Status.ClusterCFNStackID != "" ||
		ts.cfg.EKSConfig.Status.ClusterARN != "" ||
		ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint != "" ||
		ts.cfg.EKSConfig.Status.ClusterCA != "" ||
		ts.cfg.EKSConfig.Status.ClusterCADecoded != "" {
		ts.cfg.Logger.Info("non-empty cluster given; no need to create a new one", zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent))
		return nil
	}
	if ts.cfg.EKSConfig.Status.Up {
		ts.cfg.Logger.Info("cluster is up; no need to create cluster")
		return nil
	}

	ts.describeCluster()
	if ts.cfg.EKSConfig.Status.ClusterStatusCurrent == aws_eks.ClusterStatusActive {
		ts.cfg.Logger.Info("cluster status is active; no need to create cluster", zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent))
		return nil
	}

	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.Status.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	initialWait := 9 * time.Minute

	subnets := make([]string, len(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs))
	copy(subnets, ts.cfg.EKSConfig.Parameters.PublicSubnetIDs)
	if len(ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs) > 0 {
		subnets = append(subnets, ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs...)
	}

	if ts.cfg.EKSConfig.Parameters.ResolverURL != "" ||
		(ts.cfg.EKSConfig.Parameters.RequestHeaderKey != "" &&
			ts.cfg.EKSConfig.Parameters.RequestHeaderValue != "") {
		ts.cfg.Logger.Info("creating a cluster using EKS API",
			zap.String("name", ts.cfg.EKSConfig.Name),
			zap.String("resolver-url", ts.cfg.EKSConfig.Parameters.ResolverURL),
			zap.String("signing-name", ts.cfg.EKSConfig.Parameters.SigningName),
			zap.String("request-header-key", ts.cfg.EKSConfig.Parameters.RequestHeaderKey),
			zap.String("request-header-value", ts.cfg.EKSConfig.Parameters.RequestHeaderValue),
		)
		createInput := aws_eks.CreateClusterInput{
			Name:    aws.String(ts.cfg.EKSConfig.Name),
			Version: aws.String(ts.cfg.EKSConfig.Parameters.Version),
			RoleArn: aws.String(ts.cfg.EKSConfig.Parameters.RoleARN),
			ResourcesVpcConfig: &aws_eks.VpcConfigRequest{
				SubnetIds:        aws.StringSlice(subnets),
				SecurityGroupIds: aws.StringSlice([]string{ts.cfg.EKSConfig.Status.ClusterControlPlaneSecurityGroupID}),
			},
			Tags: map[string]*string{
				"Kind":                   aws.String("aws-k8s-tester"),
				"aws-k8s-tester-version": aws.String(version.ReleaseVersion),
			},
		}
		for k, v := range ts.cfg.EKSConfig.Parameters.Tags {
			createInput.Tags[k] = aws.String(v)
			ts.cfg.Logger.Info("added EKS tag to EKS API request",
				zap.String("key", k),
				zap.String("value", v),
			)
		}
		if ts.cfg.EKSConfig.Parameters.EncryptionCMKARN != "" {
			ts.cfg.Logger.Info("added encryption to EKS API request",
				zap.String("cmk-arn", ts.cfg.EKSConfig.Parameters.EncryptionCMKARN),
			)
			createInput.EncryptionConfig = []*aws_eks.EncryptionConfig{
				{
					Resources: aws.StringSlice([]string{"secrets"}),
					Provider: &aws_eks.Provider{
						KeyArn: aws.String(ts.cfg.EKSConfig.Parameters.EncryptionCMKARN),
					},
				},
			}
		}
		req, _ := ts.cfg.EKSAPI.CreateClusterRequest(&createInput)
		if ts.cfg.EKSConfig.Parameters.RequestHeaderKey != "" && ts.cfg.EKSConfig.Parameters.RequestHeaderValue != "" {
			req.HTTPRequest.Header[ts.cfg.EKSConfig.Parameters.RequestHeaderKey] = []string{ts.cfg.EKSConfig.Parameters.RequestHeaderValue}
			ts.cfg.Logger.Info("set request header for EKS create request",
				zap.String("key", ts.cfg.EKSConfig.Parameters.RequestHeaderKey),
				zap.String("value", ts.cfg.EKSConfig.Parameters.RequestHeaderValue),
			)
		}
		err = req.Send()
		if err != nil {
			return err
		}
		ts.cfg.Logger.Info("sent create cluster request")

	} else {

		tpl := template.Must(template.New("TemplateCluster").Parse(TemplateCluster))
		buf := bytes.NewBuffer(nil)
		if err := tpl.Execute(buf, templateEKSCluster{
			AWSEncryptionProviderCMKARN: ts.cfg.EKSConfig.Parameters.EncryptionCMKARN,
		}); err != nil {
			return err
		}
		tmpl := buf.String()

		initialWait = time.Minute
		ts.cfg.Logger.Info("creating a cluster using CFN", zap.String("name", ts.cfg.EKSConfig.Name))
		stackInput := &cloudformation.CreateStackInput{
			StackName:    aws.String(ts.cfg.EKSConfig.Name + "-cluster"),
			Capabilities: aws.StringSlice([]string{"CAPABILITY_IAM"}),
			OnFailure:    aws.String(cloudformation.OnFailureDelete),
			TemplateBody: aws.String(tmpl),
			Tags: cfn.NewTags(map[string]string{
				"Kind":                   "aws-k8s-tester",
				"Name":                   ts.cfg.EKSConfig.Name,
				"aws-k8s-tester-version": version.ReleaseVersion,
			}),
			Parameters: []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String("ClusterName"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.Name),
				},
				{
					ParameterKey:   aws.String("Version"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.Parameters.Version),
				},
				{
					ParameterKey:   aws.String("RoleARN"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.Parameters.RoleARN),
				},
				{
					ParameterKey:   aws.String("SubnetIDs"),
					ParameterValue: aws.String(strings.Join(subnets, ",")),
				},
				{
					ParameterKey:   aws.String("ClusterControlPlaneSecurityGroupID"),
					ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterControlPlaneSecurityGroupID),
				},
			},
		}
		if ts.cfg.EKSConfig.Parameters.EncryptionCMKARN != "" {
			ts.cfg.Logger.Info("added encryption config to EKS CFN request",
				zap.String("cmk-arn", ts.cfg.EKSConfig.Parameters.EncryptionCMKARN),
			)
			stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
				ParameterKey:   aws.String("AWSEncryptionProviderCMKARN"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Parameters.EncryptionCMKARN),
			})
		}
		stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
		if err != nil {
			return err
		}
		ts.cfg.EKSConfig.Status.ClusterCFNStackID = aws.StringValue(stackOutput.StackId)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := cfn.Poll(
			ctx,
			ts.cfg.Stopc,
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			ts.cfg.EKSConfig.Status.ClusterCFNStackID,
			cloudformation.ResourceStatusCreateComplete,
			9*time.Minute,
			30*time.Second,
		)
		var st cfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create cluster (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
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
				ts.cfg.EKSConfig.Status.ClusterARN = aws.StringValue(o.OutputValue)
			case "ClusterAPIServerEndpoint":
				ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = aws.StringValue(o.OutputValue)
			default:
				return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.Status.ClusterCFNStackID)
			}
		}

	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	ch := Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		aws_eks.ClusterStatusActive,
		initialWait,
		30*time.Second,
	)
	for v := range ch {
		ts.updateClusterStatus(v, aws_eks.ClusterStatusActive)
		err = v.Error
	}
	cancel()

	switch err {
	case nil:
		ts.cfg.Logger.Info("created a cluster",
			zap.String("cluster-cfn-stack-id", ts.cfg.EKSConfig.Status.ClusterCFNStackID),
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.Int("cluster-ca-bytes", len(ts.cfg.EKSConfig.Status.ClusterCA)),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)

	case context.DeadlineExceeded:
		ts.cfg.Logger.Warn("cluster creation took too long",
			zap.String("cluster-cfn-stack-id", ts.cfg.EKSConfig.Status.ClusterCFNStackID),
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		return err

	default:
		ts.cfg.Logger.Warn("failed to create cluster",
			zap.String("cluster-cfn-stack-id", ts.cfg.EKSConfig.Status.ClusterCFNStackID),
			zap.String("cluster-arn", ts.cfg.EKSConfig.Status.ClusterARN),
			zap.String("cluster-api-server-endpoint", ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint),
			zap.String("config-path", ts.cfg.EKSConfig.ConfigPath),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
			zap.Error(err),
		)
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteEKS() error {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************[default]\n")
		colorstring.Printf("[light_blue]deleteEKS [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("deleteEKS (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	ts.describeCluster()
	if ts.cfg.EKSConfig.Status.ClusterStatusCurrent == "" || ts.cfg.EKSConfig.Status.ClusterStatusCurrent == eksconfig.ClusterStatusDELETEDORNOTEXIST {
		ts.cfg.Logger.Info("cluster already deleted; no need to delete cluster")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.Status.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("deleting cluster", zap.String("cluster-name", ts.cfg.EKSConfig.Name))
	if ts.cfg.EKSConfig.Status.ClusterCFNStackID != "" {

		_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(ts.cfg.EKSConfig.Status.ClusterCFNStackID),
		})
		if err != nil {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", err))
			ts.cfg.Logger.Warn("failed to delete cluster", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.Status.Up = false
		ts.cfg.EKSConfig.Sync()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		ch := cfn.Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.CFNAPI,
			ts.cfg.EKSConfig.Status.ClusterCFNStackID,
			cloudformation.ResourceStatusDeleteComplete,
			5*time.Minute,
			20*time.Second,
		)
		var st cfn.StackStatus
		for st = range ch {
			if st.Error != nil {
				cancel()
				ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", st.Error))
				ts.cfg.Logger.Warn("polling errror", zap.Error(st.Error))
			}
		}
		cancel()
		if st.Error != nil {
			return st.Error
		}
		ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)

	} else {

		_, err := ts.cfg.EKSAPI.DeleteCluster(&aws_eks.DeleteClusterInput{
			Name: aws.String(ts.cfg.EKSConfig.Name),
		})
		if err != nil {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete cluster (%v)", err))
			ts.cfg.Logger.Warn("failed to delete cluster", zap.Error(err))
			return err
		}
		ts.cfg.EKSConfig.Status.Up = false
		ts.cfg.EKSConfig.Sync()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		csCh := Poll(
			ctx,
			make(chan struct{}), // do not exit on stop
			ts.cfg.Logger,
			ts.cfg.EKSAPI,
			ts.cfg.EKSConfig.Name,
			eksconfig.ClusterStatusDELETEDORNOTEXIST,
			5*time.Minute,
			20*time.Second,
		)
		for v := range csCh {
			ts.updateClusterStatus(v, eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		cancel()
	}

	ts.cfg.Logger.Info("deleted a cluster",
		zap.String("cluster-cfn-stack-id", ts.cfg.EKSConfig.Status.ClusterCFNStackID),
		zap.String("cluster-name", ts.cfg.EKSConfig.Name),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) describeCluster() {
	dout, err := ts.cfg.EKSAPI.DescribeCluster(&aws_eks.DescribeClusterInput{Name: aws.String(ts.cfg.EKSConfig.Name)})
	if err != nil {
		if isClusterDeleted(err) {
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		} else {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to describe cluster (%v)", err))
		}
	}
	if dout.Cluster == nil {
		ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
	} else {
		ts.cfg.EKSConfig.RecordStatus(aws.StringValue(dout.Cluster.Status))
	}
	ts.cfg.Logger.Info("described cluster",
		zap.String("name", ts.cfg.EKSConfig.Name),
		zap.String("status", ts.cfg.EKSConfig.Status.ClusterStatusCurrent),
	)
}

func (ts *tester) updateClusterStatus(v ClusterStatus, desired string) {
	if v.Cluster == nil {
		if v.Error != nil {
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed with error %v", v.Error))
			ts.cfg.EKSConfig.Status.Up = false
		} else {
			ts.cfg.EKSConfig.RecordStatus(eksconfig.ClusterStatusDELETEDORNOTEXIST)
		}
		return
	}
	ts.cfg.EKSConfig.Status.ClusterARN = aws.StringValue(v.Cluster.Arn)
	ts.cfg.EKSConfig.RecordStatus(aws.StringValue(v.Cluster.Status))

	if desired != eksconfig.ClusterStatusDELETEDORNOTEXIST && ts.cfg.EKSConfig.Status.ClusterStatusCurrent != eksconfig.ClusterStatusDELETEDORNOTEXIST {

		if v.Cluster.Endpoint != nil {
			ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = aws.StringValue(v.Cluster.Endpoint)
		}

		if v.Cluster.Identity != nil &&
			v.Cluster.Identity.Oidc != nil &&
			v.Cluster.Identity.Oidc.Issuer != nil {
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = aws.StringValue(v.Cluster.Identity.Oidc.Issuer)
			u, err := url.Parse(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.cfg.Logger.Warn(
					"failed to parse ClusterOIDCIssuerURL",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			if u.Scheme != "https" {
				ts.cfg.Logger.Warn("invalid scheme", zap.String("scheme", u.Scheme))
			}
			if u.Port() == "" {
				ts.cfg.Logger.Info("updating host with port :443", zap.String("host", u.Host))
				u.Host += ":443"
			}
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = u.String()
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = u.Hostname() + u.Path
			ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = fmt.Sprintf(
				"arn:aws:iam::%s:oidc-provider/%s",
				ts.cfg.EKSConfig.Status.AWSAccountID,
				ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath,
			)

			ts.cfg.Logger.Info("fetching OIDC CA thumbprint", zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL))
			httpClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{},
					Proxy:           http.ProxyFromEnvironment,
				},
			}
			resp, err := httpClient.Get(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL)
			if err != nil {
				ts.cfg.Logger.Warn("failed to fetch OIDC CA thumbprint",
					zap.String("url", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
					zap.Error(err),
				)
			}
			defer resp.Body.Close()

			if resp.TLS != nil {
				certs := len(resp.TLS.PeerCertificates)
				if certs >= 1 {
					root := resp.TLS.PeerCertificates[certs-1]
					ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = fmt.Sprintf("%x", sha1.Sum(root.Raw))
					ts.cfg.Logger.Info("fetched OIDC CA thumbprint")
				} else {
					ts.cfg.Logger.Warn("received empty TLS peer certs")
				}
			} else {
				ts.cfg.Logger.Warn("received empty HTTP empty TLS response")
			}
		}

		if v.Cluster.CertificateAuthority != nil && v.Cluster.CertificateAuthority.Data != nil {
			ts.cfg.EKSConfig.Status.ClusterCA = aws.StringValue(v.Cluster.CertificateAuthority.Data)
		}
		d, err := base64.StdEncoding.DecodeString(ts.cfg.EKSConfig.Status.ClusterCA)
		if err != nil {
			ts.cfg.Logger.Warn("failed to decode cluster CA", zap.Error(err))
		}
		ts.cfg.EKSConfig.Status.ClusterCADecoded = string(d)

	} else {

		ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = ""
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint = ""
		ts.cfg.EKSConfig.Status.ClusterCA = ""
		ts.cfg.EKSConfig.Status.ClusterCADecoded = ""

	}

	ts.cfg.EKSConfig.Sync()
}

// isClusterDeleted returns true if error from EKS API indicates that
// the EKS cluster has already been deleted.
func isClusterDeleted(err error) bool {
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

// ClusterStatus represents the EKS cluster status.
type ClusterStatus struct {
	Cluster *aws_eks.Cluster
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
		zap.String("desired-status", desiredClusterStatus),
	)

	now := time.Now()

	ch := make(chan ClusterStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		waitDur := time.Duration(0)

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

			case <-time.After(waitDur):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if waitDur == time.Duration(0) {
					waitDur = wait
				}
			}

			output, err := eksAPI.DescribeCluster(&aws_eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil {
				if isClusterDeleted(err) {
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
				zap.String("status", currentStatus),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
			switch currentStatus {
			case desiredClusterStatus:
				ch <- ClusterStatus{Cluster: cluster, Error: nil}
				lg.Info("desired cluster status; done", zap.String("status", currentStatus))
				close(ch)
				return
			case aws_eks.ClusterStatusFailed:
				ch <- ClusterStatus{Cluster: cluster, Error: fmt.Errorf("unexpected cluster status %q", aws_eks.ClusterStatusFailed)}
				lg.Warn("cluster status failed", zap.String("status", currentStatus), zap.String("desired-status", desiredClusterStatus))
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

type kubeconfig struct {
	ClusterAPIServerEndpoint string
	ClusterCA                string
	AWSIAMAuthenticatorPath  string
	ClusterName              string
}

const tmplKUBECONFIG = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: {{ .ClusterAPIServerEndpoint }}
    certificate-authority-data: {{ .ClusterCA }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: {{ .AWSIAMAuthenticatorPath }}
      args:
      - token
      - -i
      - {{ .ClusterName }}
`

// https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html
// https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
// "aws eks update-kubeconfig --name --role-arn --kubeconfig"
func (ts *tester) createClient() (cli k8s_client.EKS, err error) {
	if ts.cfg.EKSConfig.LogColor {
		colorstring.Printf("\n\n[yellow]*********************************[default]\n")
		colorstring.Printf("[light_green]createClient [default](%q)\n", ts.cfg.EKSConfig.ConfigPath)
	} else {
		fmt.Printf("\n\n*********************************\n")
		fmt.Printf("createClient (%q)\n", ts.cfg.EKSConfig.ConfigPath)
	}

	if ts.cfg.EKSConfig.AWSIAMAuthenticatorPath != "" && ts.cfg.EKSConfig.AWSIAMAuthenticatorDownloadURL != "" {
		tpl := template.Must(template.New("tmplKUBECONFIG").Parse(tmplKUBECONFIG))
		buf := bytes.NewBuffer(nil)
		if err = tpl.Execute(buf, kubeconfig{
			ClusterAPIServerEndpoint: ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint,
			ClusterCA:                ts.cfg.EKSConfig.Status.ClusterCA,
			AWSIAMAuthenticatorPath:  ts.cfg.EKSConfig.AWSIAMAuthenticatorPath,
			ClusterName:              ts.cfg.EKSConfig.Name,
		}); err != nil {
			return nil, err
		}
		ts.cfg.Logger.Info("writing KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
		if err = ioutil.WriteFile(ts.cfg.EKSConfig.KubeConfigPath, buf.Bytes(), 0777); err != nil {
			return nil, err
		}
		ts.cfg.Logger.Info("wrote KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
	} else {
		args := []string{
			ts.cfg.EKSConfig.AWSCLIPath,
			"eks",
			fmt.Sprintf("--region=%s", ts.cfg.EKSConfig.Region),
			"update-kubeconfig",
			fmt.Sprintf("--name=%s", ts.cfg.EKSConfig.Name),
			fmt.Sprintf("--kubeconfig=%s", ts.cfg.EKSConfig.KubeConfigPath),
			"--verbose",
		}
		if ts.cfg.EKSConfig.Parameters.ResolverURL != "" {
			args = append(args, fmt.Sprintf("--endpoint=%s", ts.cfg.EKSConfig.Parameters.ResolverURL))
		}
		cmd := strings.Join(args, " ")

		ts.cfg.Logger.Info("writing KUBECONFIG with 'aws eks update-kubeconfig'",
			zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath),
			zap.String("cmd", cmd),
		)

		retryStart, waitDur := time.Now(), 3*time.Minute
		var output []byte
		for time.Now().Sub(retryStart) < waitDur {
			select {
			case <-ts.cfg.Stopc:
				return nil, errors.New("update-kubeconfig aborted")
			case <-time.After(5 * time.Second):
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
			cancel()
			out := string(output)
			if err != nil {
				ts.cfg.Logger.Warn("'aws eks update-kubeconfig' failed", zap.String("output", out), zap.Error(err))
				if !strings.Contains(out, "Cluster status not active") || !strings.Contains(err.Error(), "exit") {
					return nil, fmt.Errorf("'aws eks update-kubeconfig' failed (output %q, error %v)", out, err)
				}
				continue
			}
			ts.cfg.Logger.Info("'aws eks update-kubeconfig' success", zap.String("output", out), zap.String("kubeconfig-path", ts.cfg.EKSConfig.KubeConfigPath))
			break
		}
		if err != nil {
			ts.cfg.Logger.Warn("failed 'aws eks update-kubeconfig'", zap.String("output", string(output)), zap.Error(err))
			return nil, err
		}

		ts.cfg.Logger.Info("ran 'aws eks update-kubeconfig'")
		fmt.Printf("\n\n'%s' output:\n\n%s\n\n", cmd, strings.TrimSpace(string(output)))
	}

	ts.cfg.Logger.Info("creating k8s client")
	kcfg := &k8s_client.EKSConfig{
		Logger:            ts.cfg.Logger,
		Region:            ts.cfg.EKSConfig.Region,
		ClusterName:       ts.cfg.EKSConfig.Name,
		KubeConfigPath:    ts.cfg.EKSConfig.KubeConfigPath,
		KubectlPath:       ts.cfg.EKSConfig.KubectlPath,
		ServerVersion:     ts.cfg.EKSConfig.Parameters.Version,
		EncryptionEnabled: ts.cfg.EKSConfig.Parameters.EncryptionCMKARN != "",
		Clients:           ts.cfg.EKSConfig.Clients,
		ClientQPS:         ts.cfg.EKSConfig.ClientQPS,
		ClientBurst:       ts.cfg.EKSConfig.ClientBurst,
		ClientTimeout:     ts.cfg.EKSConfig.ClientTimeout,
	}
	if ts.cfg.EKSConfig.Status != nil {
		kcfg.ClusterAPIServerEndpoint = ts.cfg.EKSConfig.Status.ClusterAPIServerEndpoint
		kcfg.ClusterCADecoded = ts.cfg.EKSConfig.Status.ClusterCADecoded
	}
	cli, err = k8s_client.NewEKS(kcfg)
	if err != nil {
		ts.cfg.Logger.Warn("failed to create k8s client", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("created k8s client")
	}
	return cli, err
}
