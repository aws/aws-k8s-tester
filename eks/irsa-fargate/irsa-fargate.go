// Package irsafargate implements tester for
// "IAM Roles for Service Accounts (IRSA)" Pod with Fargate.
package irsafargate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eks/fargate"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines "IAM Roles for Service Accounts (IRSA)" for Fargate configuration.
// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	CFNAPI    cloudformationiface.CloudFormationAPI
	EKSAPI    eksiface.EKSAPI
	IAMAPI    iamiface.IAMAPI
	S3API     s3iface.S3API
}

// Tester defines Fargate tester.
type Tester interface {
	// Create creates Fargate pods.
	Create() error
	// Delete deletes Fargate pods.
	Delete() error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnIRSAFargate.Created {
		ts.cfg.Logger.Info("skipping create AddOnIRSAFargate")
		return nil
	}

	ts.cfg.EKSConfig.AddOnIRSAFargate.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnIRSAFargate.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnIRSAFargate.CreateTookString = ts.cfg.EKSConfig.AddOnIRSAFargate.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace); err != nil {
		return err
	}
	if err := ts.createS3(); err != nil {
		return err
	}
	if err := ts.createOIDCProvider(); err != nil {
		return err
	}
	if err := ts.createRole(); err != nil {
		return err
	}
	if err := ts.createServiceAccount(); err != nil {
		return err
	}
	if err := ts.createConfigMap(); err != nil {
		return err
	}
	if err := ts.createProfile(); err != nil {
		return err
	}
	if err := ts.createPod(); err != nil {
		return err
	}
	if err := ts.checkPod(); err != nil {
		return err
	}
	if err := ts.checkNode(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnIRSAFargate.Created {
		ts.cfg.Logger.Info("skipping delete AddOnIRSAFargate")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnIRSAFargate.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnIRSAFargate.DeleteTookString = ts.cfg.EKSConfig.AddOnIRSAFargate.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deletePod(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate Pod (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting Fargate Pod")

	if err := ts.deleteProfile(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate profile (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting Fargate profile")
	time.Sleep(10 * time.Second)

	if err := ts.deleteConfigMaps(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ConfigMap (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting ConfigMap")
	time.Sleep(20 * time.Second)

	if err := ts.deleteServiceAccount(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete ServiceAccount (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting ServiceAccount")
	time.Sleep(20 * time.Second)

	if err := ts.deleteRole(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete IRSA IAM Role (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting IAM Role")
	time.Sleep(20 * time.Second)

	if err := ts.deleteOIDCProvider(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete IAM Open ID Connect provider (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting OIDC provider")
	time.Sleep(time.Minute)

	if err := ts.deleteS3(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete S3 (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting S3")
	time.Sleep(20 * time.Second)

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnIRSAFargate.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createS3() (err error) {
	if ts.cfg.EKSConfig.S3BucketName == "" {
		return errors.New("empty S3 bucket name for IRSA add-on")
	}
	_, err = ts.cfg.S3API.PutObject(&s3.PutObjectInput{
		Bucket:  aws.String(ts.cfg.EKSConfig.S3BucketName),
		Key:     aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key),
		Body:    bytes.NewReader(randBytes(1024)),
		Expires: aws.Time(time.Now().Add(24 * time.Hour)),

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
		},
	})
	if err == nil {
		ts.cfg.Logger.Info("uploaded",
			zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
			zap.String("remote-path", ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key),
		)
	} else {
		ts.cfg.Logger.Warn("failed to upload",
			zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
			zap.String("remote-path", ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key),
			zap.Error(err),
		)
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteS3() error {
	if ts.cfg.EKSConfig.S3BucketName == "" {
		ts.cfg.Logger.Info("skipping S3 deletes for IRSA add-on")
		return nil
	}
	s3Key := ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key
	_, err := ts.cfg.S3API.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(ts.cfg.EKSConfig.S3BucketName),
		Key:    aws.String(s3Key),
	})
	if err == nil {
		ts.cfg.Logger.Info("deleted the private key in S3",
			zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
			zap.String("remote-path", s3Key),
		)
	} else {
		ts.cfg.Logger.Warn("failed to delete the private key in S3",
			zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
			zap.String("remote-path", s3Key),
			zap.Error(err),
		)
	}
	return err
}

func (ts *tester) createOIDCProvider() error {
	if ts.cfg.EKSConfig.Name == "" {
		return errors.New("EKSConfig.Name is empty")
	}
	if ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL == "" {
		return errors.New("EKSConfig.Status.ClusterOIDCIssuerURL is empty")
	}

	ts.cfg.Logger.Info("checking existing IAM Open ID Connect provider",
		zap.String("provider-arn", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
	)
	needCreate := false
	_, err := ts.cfg.IAMAPI.GetOpenIDConnectProvider(&iam.GetOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
	})
	if err != nil {
		arr := err.(awserr.Error)
		if arr.Code() == iam.ErrCodeNoSuchEntityException {
			needCreate = true
		} else {
			ts.cfg.Logger.Warn("failed to get IAM Open ID Connect provider", zap.Error(err))
		}
	} else {
		ts.cfg.Logger.Info("IAM Open ID Connect provider already exists",
			zap.String("provider-arn", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
		)
	}

	if needCreate {
		ts.cfg.Logger.Info("creating IAM Open ID Connect provider")
		output, err := ts.cfg.IAMAPI.CreateOpenIDConnectProvider(&iam.CreateOpenIDConnectProviderInput{ // no name or tags, keyed to URL
			Url:            aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerURL),
			ThumbprintList: aws.StringSlice([]string{ts.cfg.EKSConfig.Status.ClusterOIDCIssuerCAThumbprint}),
			ClientIDList:   aws.StringSlice([]string{"sts.amazonaws.com"}),
		})
		if err != nil {
			return err
		}
		ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN = aws.StringValue(output.OpenIDConnectProviderArn)
		ts.cfg.EKSConfig.Sync()
		ts.cfg.Logger.Info("created IAM Open ID Connect provider", zap.String("provider-arn", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN))
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteOIDCProvider() error {
	ts.cfg.Logger.Info("deleting IAM Open ID Connect provider",
		zap.String("provider-arn", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
	)
	_, err := ts.cfg.IAMAPI.DeleteOpenIDConnectProvider(&iam.DeleteOpenIDConnectProviderInput{
		OpenIDConnectProviderArn: aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
	})
	if err != nil {
		arr := err.(awserr.Error)
		if arr.Code() == iam.ErrCodeNoSuchEntityException {
			ts.cfg.Logger.Warn("IAM Open ID Connect provider already deleted", zap.Error(err))
		} else {
			ts.cfg.Logger.Warn("failed to delete IAM Open ID Connect provider", zap.Error(err))
		}
	} else {
		ts.cfg.Logger.Info("deleted IAM Open ID Connect provider",
			zap.String("provider-arn", ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
		)
	}
	return ts.cfg.EKSConfig.Sync()
}

// TemplateRole is the CloudFormation template for EKS IRSA Fargate role.
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster IRSA Fargate Role'

Parameters:

  RoleName:
    Type: String
    Description: The name of the IRSA Fargate role

  IssuerARN:
    Type: String
    Description: EKS IRSA Fargate Provider ARN

  Namespace:
    Type: String
    Description: The namespace for the IRSA Fargate role

  ServiceAccountName:
    Type: String
    Description: The ServiceAccount name for the IRSA Fargate role

  RoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'eks.amazonaws.com,eks-fargate-pods.amazonaws.com'
    Description: EKS Fargate Role Service Principals

  RoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess,arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy'
    Description: EKS IRSA Fargate policy ARNs

Resources:

  Role:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref RoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Federated: !Ref IssuerARN
          Action:
          - sts:AssumeRoleWithWebIdentity
          Condition:
            StringEquals:
              {{ .IRSAIssuerHostPath }}:sub: !Join [':', ['system:serviceaccount', !Ref Namespace, !Ref ServiceAccountName]]
        - Effect: Allow
          Principal:
            Service: !Ref RoleServicePrincipals
          Action:
          - sts:AssumeRole
      ManagedPolicyArns: !Ref RoleManagedPolicyARNs
      Path: /


Outputs:

  RoleARN:
    Description: The IRSA Fargate role ARN
    Value: !GetAtt Role.Arn

`

type irsaTemplate struct {
	IRSAIssuerHostPath string
}

func (ts *tester) createRole() error {
	if ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName == "" {
		return errors.New("empty AddOnIRSAFargate.RoleName")
	}
	if ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID != "" ||
		ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN != "" {
		ts.cfg.Logger.Info("non-empty IRSA roleARN given; no need to create a new one")
		return nil
	}

	tpl := template.Must(template.New("TemplateRole").Parse(TemplateRole))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, irsaTemplate{
		IRSAIssuerHostPath: ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ts.cfg.Logger.Info("creating a new IRSA role using CFN", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName))
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(tplTxt),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("RoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName),
			},
			{
				ParameterKey:   aws.String("IssuerARN"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
			},
			{
				ParameterKey:   aws.String("Namespace"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace),
			},
			{
				ParameterKey:   aws.String("ServiceAccountName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName),
			},
		},
	}
	if len(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new IRSA role with role service principals",
			zap.Strings("role-service-principals", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new IRSA role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleManagedPolicyARNs, ",")),
		})
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create IRSA Fargate role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "RoleARN":
			ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a IRSA role",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID),
		zap.String("role-name", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteRole() error {
	if ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty IRSA role CFN stack ID; no need to delete IRSA")
		return nil
	}

	ts.cfg.Logger.Info("deleting IRSA role CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		make(chan struct{}), // do not exit on stop

		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete IRSA role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a IRSA role",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating service account", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
					Labels: map[string]string{
						"name": ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName,
					},
					Annotations: map[string]string{
						"eks.amazonaws.com/role-arn": ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created service account", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting service account", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted service account", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

// TemplateConfigMap is the IRSA config map.
const TemplateConfigMap = `
#!/usr/bin/env bash
set -e
printf "\nInstalling AWS CLI...\n"
yum install -y -q python3-pip
pip3 install --upgrade --quiet awscli
printf "\nAWS CLI version:\n"
aws --version
printf "\nProjected ServiceAccount token:\n"
cat $AWS_WEB_IDENTITY_TOKEN_FILE; echo
printf "\nHOSTNAME:\n"
echo $HOSTNAME
printf "\nAWS_ROLE_ARN: "
echo $AWS_ROLE_ARN
printf "\n'aws sts get-caller-identity' output:\n"
aws sts get-caller-identity
CALLER_ROLE_ARN=$(aws sts get-caller-identity --query Arn --output text)
printf "\n'aws sts get-caller-identity' role ARN:\n"
echo $CALLER_ROLE_ARN
if [[ $CALLER_ROLE_ARN =~ *{{ .RoleName }}* ]]; then
  echo "Unexpected CALLER_ROLE_ARN: ${CALLER_ROLE_ARN}"
  exit 1
fi
printf "\nSUCCESS IRSA FARGATE TEST: CALLER_ROLE_ARN FOUND!\n\n"
aws s3 cp s3://{{ .S3BucketName }}/{{ .S3Key }} /tmp/{{ .S3Key }}
printf "\n"
echo {{ .S3Key }} contents:
cat /tmp/{{ .S3Key }}
printf "\nSUCCESS IRSA FARGATE TEST: S3 FILE DOWNLOADED!\n\n"
printf "\n{{ .SleepMessage }}\n\n"
sleep 86400
printf "\nSUCCESS IRSA FARGATE TEST: EXITING...\n\n"
`

const sleepMsg = `SUCCESS IRSA FARGATE TEST: SLEEPING...`

type configMapTemplate struct {
	RoleName     string
	S3BucketName string
	S3Key        string
	SleepMessage string
}

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("creating IRSA config map", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName))

	tpl := template.Must(template.New("TemplateConfigMap").Parse(TemplateConfigMap))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, configMapTemplate{
		RoleName:     ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName,
		S3BucketName: ts.cfg.EKSConfig.S3BucketName,
		S3Key:        ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key,
		SleepMessage: sleepMsg,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
					Labels: map[string]string{
						"name": ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
					},
				},
				Data: map[string]string{
					ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapScriptFileName: tplTxt,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created IRSA config map", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMaps() error {
	ts.cfg.Logger.Info("deleting config maps", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("deleted config map", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createProfile() error {
	if ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN == "" {
		return errors.New("empty AddOnIRSAFargate.RoleARN")
	}
	if len(ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs) == 0 {
		return errors.New("empty Parameters.PrivateSubnetIDs")
	}
	ts.cfg.Logger.Info("creating fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName))

	req, _ := ts.cfg.EKSAPI.CreateFargateProfileRequest(&eks.CreateFargateProfileInput{
		ClusterName:         aws.String(ts.cfg.EKSConfig.Name),
		FargateProfileName:  aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName),
		PodExecutionRoleArn: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN),
		Subnets:             aws.StringSlice(ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs),
		Selectors: []*eks.FargateProfileSelector{
			{
				Namespace: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace),
			},
		},
	})
	err := req.Send()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("sent create fargate profile request")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	ch := fargate.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName,
		eks.FargateProfileStatusActive,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		if sv.Error != nil {
			cancel()
			return sv.Error
		}
	}
	cancel()

	ts.cfg.Logger.Info("created fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteProfile() error {
	ts.cfg.Logger.Info("deleting fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName))

	var err error
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < time.Minute {
		_, err = ts.cfg.EKSAPI.DeleteFargateProfile(&eks.DeleteFargateProfileInput{
			ClusterName:        aws.String(ts.cfg.EKSConfig.Name),
			FargateProfileName: aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName),
		})
		if err != nil {
			if fargate.IsProfileDeleted(err) {
				err = nil
				break
			}
			ts.cfg.Logger.Warn("failed to delete fargate profile; retrying", zap.Error(err))
			select {
			case <-ts.cfg.Stopc:
				ts.cfg.Logger.Warn("aborted")
				return nil
			case <-time.After(5 * time.Second):
			}
			continue
		}
		ts.cfg.Logger.Warn("requested to delete fargate profile")
		break
	}

	ch := fargate.Poll(
		context.Background(),
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName,
		fargate.FargateProfileStatusDELETEDORNOTEXIST,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		if sv.Error != nil {
			return sv.Error
		}
	}

	ts.cfg.Logger.Info("deleted fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkNode() error {
	ts.cfg.Logger.Info("checking node")

	desired := 1
	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("aborted")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items := nodes.Items

		readies := 0
		for _, node := range items {
			labels := node.GetLabels()
			nodeName := node.GetName()
			ts.cfg.Logger.Info("checking node-info conditions", zap.String("node-name", nodeName), zap.String("labels", fmt.Sprintf("%+v", labels)))
			for _, cond := range node.Status.Conditions {
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("node info",
					zap.String("node-name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				if cond.Status == v1.ConditionTrue && strings.HasPrefix(nodeName, "fargate-") {
					readies++
				}
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", desired),
		)
		if readies >= desired {
			break
		}
	}

	ts.cfg.Logger.Info("checked node")
	return ts.cfg.EKSConfig.Sync()
}

// TemplatePodScript is the script to run in Deployment.
const TemplatePodScript = `printf '\n\nexecuting...\n\n'; /opt/{{ .ConfigMapScriptFileName }};`

type podScriptTemplate struct {
	ConfigMapScriptFileName string
}

func (ts *tester) createPod() error {
	if err := ts.listPods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace); err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
	}

	tpl := template.Must(template.New("TemplatePodScript").Parse(TemplatePodScript))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, podScriptTemplate{
		ConfigMapScriptFileName: ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapScriptFileName,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ts.cfg.Logger.Info("creating Pod")
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ts.cfg.EKSConfig.AddOnIRSAFargate.PodName,
			Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy:      v1.RestartPolicyOnFailure,
			ServiceAccountName: ts.cfg.EKSConfig.AddOnIRSAFargate.ServiceAccountName,
			Containers: []v1.Container{
				{
					Name:  ts.cfg.EKSConfig.AddOnIRSAFargate.ContainerName,
					Image: "amazonlinux",

					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"sh",
						"-c",
						tplTxt,
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
							MountPath: "/opt",
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{ // to execute
					Name: ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: ts.cfg.EKSConfig.AddOnIRSAFargate.ConfigMapName,
							},
							DefaultMode: aws.Int32(0777),
						},
					},
				},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created Pod")
	return ts.cfg.EKSConfig.Sync()
}

var propagationBackground = metav1.DeletePropagationBackground

func (ts *tester) deletePod() error {
	ts.cfg.Logger.Info("deleting Pod", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.PodName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			ts.cfg.EKSConfig.AddOnIRSAFargate.PodName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete Pod %q (%v)", ts.cfg.EKSConfig.AddOnIRSAFargate.PodName, err)
	}
	ts.cfg.Logger.Info("deleted Pod", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.PodName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) listPods(ns string) error {
	pods, err := ts.getPods(ns)
	if err != nil {
		return err
	}
	println()
	for _, v := range pods.Items {
		fmt.Printf("%q Pod using client-go: %q\n", ns, v.Name)
	}
	println()
	return nil
}

func (ts *tester) getPods(ns string) (*v1.PodList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	ps, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	cancel()
	return ps, err
}

func (ts *tester) checkPod() error {
	argsDesc := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		"describe",
		"pods/" + ts.cfg.EKSConfig.AddOnIRSAFargate.PodName,
	}
	cmdTxtDesc := strings.Join(argsDesc, " ")
	argsLogs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		"logs",
		"pods/" + ts.cfg.EKSConfig.AddOnIRSAFargate.PodName,
		"--timestamps",
	}
	cmdTxtLogs := strings.Join(argsLogs, " ")
	ts.cfg.Logger.Info("checking Pod logs",
		zap.String("pod-name", ts.cfg.EKSConfig.AddOnIRSAFargate.PodName),
		zap.String("container-name", ts.cfg.EKSConfig.AddOnIRSAFargate.ContainerName),
		zap.String("command-describe", cmdTxtDesc),
		zap.String("command-logs", cmdTxtLogs),
	)
	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("aborted")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, argsDesc[0], argsDesc[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
		}
		fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdTxtDesc, out)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, argsLogs[0], argsLogs[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Printf("\n'%s' output:\n\n%s\n\n", cmdTxtLogs, out)

		ts.cfg.Logger.Info("checked Pod logs",
			zap.String("pod-name", ts.cfg.EKSConfig.AddOnIRSAFargate.PodName),
			zap.String("container-name", ts.cfg.EKSConfig.AddOnIRSAFargate.ContainerName),
		)

		if !strings.Contains(out, sleepMsg) {
			ts.cfg.Logger.Warn("unexpected logs output", zap.String("output", out))
			time.Sleep(5 * time.Second)
			continue
		}

		break
	}

	// TODO: fail if "sleepMsg" not found?

	return ts.cfg.EKSConfig.Sync()
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return b
}
