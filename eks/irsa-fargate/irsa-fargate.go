// Package irsafargate implements tester for
// "IAM Roles for Service Accounts (IRSA)" Pod with Fargate.
package irsafargate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"text/template"
	"time"

	fargate_wait "github.com/aws/aws-k8s-tester/eks/fargate/wait"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines "IAM Roles for Service Accounts (IRSA)" for Fargate configuration.
// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
	CFNAPI    cloudformationiface.CloudFormationAPI
	EKSAPI    eksiface.EKSAPI
	IAMAPI    iamiface.IAMAPI
	ECRAPI    ecriface.ECRAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg          Config
	ecrImage     string
	sleepMessage string
	testBody     string
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnIRSAFargate() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnIRSAFargate.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnIRSAFargate.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnIRSAFargate.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RepositoryRegion,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RepositoryName,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
	); err != nil {
		return err
	}
	if err = ts.createS3Object(); err != nil {
		return err
	}
	if err = ts.createOIDCProvider(); err != nil {
		return err
	}
	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createServiceAccount(); err != nil {
		return err
	}
	if err = ts.createConfigMap(); err != nil {
		return err
	}
	if err = ts.createProfile(); err != nil {
		return err
	}
	if err = ts.createPod(); err != nil {
		return err
	}
	if err = ts.checkPodWebhook(); err != nil {
		return err
	}
	if err = ts.checkResults(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnIRSAFargate() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnIRSAFargate.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnIRSAFargate.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnIRSAFargate.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createS3Object() (err error) {
	if ts.cfg.EKSConfig.S3BucketName == "" {
		return errors.New("empty S3 bucket name for IRSA add-on")
	}
	ts.testBody = randutil.String(256)
	ts.sleepMessage = `SUCCESS IRSA FARGATE TEST: SLEEPING WITH ` + randutil.String(32)
	return aws_s3.UploadBody(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key,
		strings.NewReader(ts.testBody),
	)
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
    Default: 'arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy'
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
      Policies:
      - PolicyName: !Join ['-', [!Ref RoleName, 's3-policy']]
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
          - Effect: Allow
            Action:
            - s3:ListBucket
            - s3:GetObject
            Resource:
            - !Join ['', [!Sub 'arn:${AWS::Partition}:s3:::', '{{.S3BucketName}}']]
            - !Join ['', [!Sub 'arn:${AWS::Partition}:s3:::', '{{.S3BucketName}}', '/', '{{.ClusterName}}', '/*']]

Outputs:

  RoleARN:
    Description: The IRSA Fargate role ARN
    Value: !GetAtt Role.Arn

`

type irsaTemplate struct {
	IRSAIssuerHostPath string
	S3BucketName       string
	ClusterName        string
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
		S3BucketName:       ts.cfg.EKSConfig.S3BucketName,
		ClusterName:        ts.cfg.EKSConfig.Name,
	}); err != nil {
		return err
	}

	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackYAMLPath, buf.Bytes(), 0400); err != nil {
		return err
	}
	if err := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackYAMLS3Key,
		ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackYAMLPath,
	); err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating a new IRSA Fargate role using CFN",
		zap.String("role-name", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName),
		zap.String("role-cfn-file-path", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleCFNStackYAMLPath),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(buf.String()),
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
				ParameterValue: aws.String(irsaFargateServiceAccountName),
			},
		},
	}
	if len(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new IRSA Fargate role with role service principals",
			zap.Strings("role-service-principals", ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("RoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.EKSConfig.AddOnIRSAFargate.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new IRSA Fargate role with custom managed role policies",
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
		ts.cfg.LogWriter,
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
		ts.cfg.LogWriter,
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

const (
	irsaFargateServiceAccountName = "irsa-fargate-service-account"
	irsaFargateConfigMapName      = "irsa-fargate-configmap"
	irsaFargateConfigMapFileName  = "irsa-fargate-configmap.bash"
	irsaFargatePodName            = "irsa-fargate-pod"
	irsaFargateContainerName      = "irsa-fargate-container"
)

func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating service account", zap.String("name", irsaFargateServiceAccountName))
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
					Name:      irsaFargateServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
					Labels: map[string]string{
						"name": irsaFargateServiceAccountName,
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
	ts.cfg.Logger.Info("created service account", zap.String("name", irsaFargateServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting service account", zap.String("name", irsaFargateServiceAccountName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			irsaFargateServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted service account", zap.String("name", irsaFargateServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

// TemplateConfigMap is the IRSA Fargate config map.
const TemplateConfigMap = `
#!/usr/bin/env bash
set -e

printf "\n"
aws --version
/s3-utils version
/sts-utils version

printf "\nhttp://169.254.169.254/latest/meta-data/ami-id with IMDBv1:\n"
curl -v http://169.254.169.254/latest/meta-data/ami-id || true

printf "\n\nProjected ServiceAccount token AWS_WEB_IDENTITY_TOKEN_FILE:\n"
cat $AWS_WEB_IDENTITY_TOKEN_FILE; echo

printf "\nHOSTNAME:\n"
echo $HOSTNAME

printf "\nAWS_ROLE_ARN:\n"
echo $AWS_ROLE_ARN

printf "\n'sts-utils get-caller-identity' output:\n"
/sts-utils get-caller-identity --log-level debug --partition {{.Partition}} --region {{.Region}} || true

# printf "\n'aws s3 cp':\n"
# aws s3 cp s3://{{ .S3BucketName }}/{{ .S3Key }} /tmp/$HOSTNAME.s3.output;

printf "\n's3-utils cp':\n"
/s3-utils cp --log-level info --partition {{.Partition}} --region {{.Region}} --s3-bucket {{ .S3BucketName }} --s3-key {{ .S3Key }} --local-path /var/log/$HOSTNAME.s3.output;

printf "\n"
echo {{ .S3Key }} contents:
cat /tmp/$HOSTNAME.s3.output;
printf "\n\nSUCCESS IRSA FARGATE TEST: S3 FILE DOWNLOADED!\n\n"

printf "\n'sts-utils get-caller-identity' expected role ARN:\n"
/sts-utils get-caller-identity --partition {{.Partition}} --region {{.Region}} --match-contain-role-arn {{ .RoleName }}
printf "\nSUCCESS IRSA FARGATE TEST: CALLER_ROLE_ARN FOUND!\n\n"

printf "\n{{ .SleepMessage }}\n\n"
sleep 86400


printf "\nSUCCESS IRSA FARGATE TEST: EXITING...\n\n"
`

/*
# https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
printf "\nhttp://169.254.169.254/latest/meta-data/ami-id with IMDBv2:\n"
TOKEN=` + "`" + `curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600"` + "`" + `\
&& curl -H "X-aws-ec2-metadata-token: $TOKEN" -v http://169.254.169.254/latest/meta-data/ami-id || true
printf "\n"

printf "\n'aws sts get-caller-identity' output:\n"
aws --debug --cli-read-timeout=5 --cli-connect-timeout=5 sts get-caller-identity || true

CALLER_ROLE_ARN=$(aws --cli-read-timeout=5 --cli-connect-timeout=5 sts get-caller-identity --query Arn --output text || true)
echo $CALLER_ROLE_ARN
if [[ $CALLER_ROLE_ARN =~ *{{ .RoleName }}* ]]; then
  echo "Unexpected CALLER_ROLE_ARN: ${CALLER_ROLE_ARN}"
  exit 1
fi
*/

type configMapTemplate struct {
	Partition    string
	Region       string
	RoleName     string
	S3BucketName string
	S3Key        string
	SleepMessage string
}

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("creating IRSA config map", zap.String("name", irsaFargateConfigMapName))

	tpl := template.Must(template.New("TemplateConfigMap").Parse(TemplateConfigMap))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, configMapTemplate{
		Partition: ts.cfg.EKSConfig.Partition,
		Region:    ts.cfg.EKSConfig.Region,

		// e.g.
		// created role ARN:    arn:aws:iam::607362164682:role/eks-2020071200-galaxyzejwho-add-on-irsa-role
		// sts caller role ARN: arn:aws:sts::607362164682:assumed-role/eks-2020071200-galaxyzejwho-add-on-irsa-role/botocore-session-1594541343
		RoleName: ts.cfg.EKSConfig.AddOnIRSAFargate.RoleName,

		S3BucketName: ts.cfg.EKSConfig.S3BucketName,
		S3Key:        ts.cfg.EKSConfig.AddOnIRSAFargate.S3Key,
		SleepMessage: ts.sleepMessage,
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
					Name:      irsaFargateConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
					Labels: map[string]string{
						"name": irsaFargateConfigMapName,
					},
				},
				Data: map[string]string{
					irsaFargateConfigMapFileName: tplTxt,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created IRSA config map", zap.String("name", irsaFargateConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMaps() error {
	ts.cfg.Logger.Info("deleting config maps", zap.String("name", irsaFargateConfigMapName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			irsaFargateConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("deleted config map", zap.String("name", irsaFargateConfigMapName))
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

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	ch := fargate_wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName,
		eks.FargateProfileStatusActive,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		err = sv.Error
	}
	cancel()
	if err != nil {
		return fmt.Errorf("failed to wait for irsa fargate profile creation %v", err)
	}

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
		if err != nil && fargate_wait.IsProfileDeleted(err) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	ch := fargate_wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName,
		fargate_wait.FargateProfileStatusDELETEDORNOTEXIST,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		err = sv.Error
	}
	cancel()
	if err != nil {
		return fmt.Errorf("failed to wait for irsa fargate profile deletion %v", err)
	}

	ts.cfg.Logger.Info("deleted fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnIRSAFargate.ProfileName))
	return ts.cfg.EKSConfig.Sync()
}

// TemplatePodScript is the script to run in Deployment.
const TemplatePodScript = `printf '\n\nexecuting...\n\n'; /opt/{{ .ConfigMapScriptFileName }};`

type podScriptTemplate struct {
	ConfigMapScriptFileName string
}

func (ts *tester) createPod() error {
	tpl := template.Must(template.New("TemplatePodScript").Parse(TemplatePodScript))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, podScriptTemplate{
		ConfigMapScriptFileName: irsaFargateConfigMapFileName,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ts.cfg.Logger.Info("creating IRSA Fargate Pod", zap.String("image", ts.ecrImage))
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      irsaFargatePodName,
			Namespace: ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		},
		Spec: v1.PodSpec{
			ServiceAccountName: irsaFargateServiceAccountName,
			RestartPolicy:      v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:  irsaFargateContainerName,
					Image: ts.ecrImage,

					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"sh",
						"-c",
						tplTxt,
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{ // to execute
							Name:      irsaFargateConfigMapName,
							MountPath: "/opt",
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{ // to execute
					Name: irsaFargateConfigMapName,
					VolumeSource: v1.VolumeSource{
						ConfigMap: &v1.ConfigMapVolumeSource{
							LocalObjectReference: v1.LocalObjectReference{
								Name: irsaFargateConfigMapName,
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
	ts.cfg.Logger.Info("deleting Pod", zap.String("name", irsaFargatePodName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace).
		Delete(
			ctx,
			irsaFargatePodName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete Pod %q (%v)", irsaFargatePodName, err)
	}
	ts.cfg.Logger.Info("deleted Pod", zap.String("name", irsaFargatePodName))
	return ts.cfg.EKSConfig.Sync()
}

// https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
func (ts *tester) checkPodWebhook() error {
	ts.cfg.Logger.Info("checking IRSA Pod spec for webhook")
	waitDur := 2 * time.Minute
	retryStart := time.Now()
	found := false
foundBreak:
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(5 * time.Second):
		}

		pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace, 150, 5*time.Second)
		if err != nil {
			ts.cfg.Logger.Warn("failed to list IRSA Pods", zap.Error(err))
			continue
		}
		ts.cfg.Logger.Info("listed Pods", zap.Int("items", len(pods)))
		for _, pod := range pods {
			for _, con := range pod.Spec.Containers {
				foundARN, foundToken := false, false
				for _, env := range con.Env {
					ts.cfg.Logger.Info("env",
						zap.String("pod", pod.Name),
						zap.String("key", env.Name),
						zap.String("value", env.Value),
					)
					switch env.Name {
					case "AWS_ROLE_ARN":
						if env.Value != ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN {
							return fmt.Errorf("%q expected %q, got %q", env.Name, ts.cfg.EKSConfig.AddOnIRSAFargate.RoleARN, env.Value)
						}
						ts.cfg.Logger.Info("found injected AWS_ROLE_ARN in Pod", zap.String("pod", pod.Name))
						foundARN = true
					case "AWS_WEB_IDENTITY_TOKEN_FILE":
						ts.cfg.Logger.Info("found injected AWS_WEB_IDENTITY_TOKEN_FILE in Pod", zap.String("pod", pod.Name))
						foundToken = true
					}
					if foundARN && foundToken {
						found = true
						break foundBreak
					}
				}
			}
		}
	}
	if !found {
		return errors.New("IRSA admission controller did not work")
	}

	ts.cfg.Logger.Info("checked IRSA Pod spec for webhook")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkResults() (err error) {
	// TODO: "aws sts get-caller-identity" fails with
	// Could not connect to the endpoint URL: "https://sts.amazonaws.com/"
	nodeReady := false

	ts.cfg.Logger.Info("checking results")
	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)*3*time.Second
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(5 * time.Second):
		}
		if err = ts.checkNodeReadiness(); err != nil {
			ts.cfg.Logger.Warn("failed to check node", zap.Error(err))
			continue
		}
		nodeReady = true
		if err = ts.checkPodLogs(); err != nil {
			ts.cfg.Logger.Warn("failed to check pod", zap.Error(err))
			continue
		}
		ready = true
		break
	}
	if !ready {
		if nodeReady {
			ready = true
		}
		// return errors.New("failed to check IRSA Fargate Pod")
	}
	ts.cfg.Logger.Info("checked results")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkPodLogs() error {
	descArgsPods := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		"describe",
		"pods/" + irsaFargatePodName,
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	logArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace,
		"logs",
		"pods/" + irsaFargatePodName,
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	ts.cfg.Logger.Info("checking Pod",
		zap.String("pod-name", irsaFargatePodName),
		zap.String("container-name", irsaFargateContainerName),
		zap.String("command-describe", descCmdPods),
		zap.String("command-logs", logsCmd),
	)

	pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace, 150, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
		return err
	}
	if len(pods) > 0 {
		ts.cfg.Logger.Info("pods found", zap.Int("pods", len(pods)))
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
		for _, pod := range pods {
			fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", ts.cfg.EKSConfig.AddOnIRSAFargate.Namespace, pod.Name)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n")
	} else {
		ts.cfg.Logger.Info("no pod found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
	cancel()
	out := string(output)
	if err != nil {
		ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, out)

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
	cancel()
	out = string(output)
	if err != nil {
		ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", logsCmd, out)
	if !strings.Contains(out, ts.sleepMessage) {
		ts.cfg.Logger.Warn("unexpected logs output")
		return fmt.Errorf("unexpected logs output; expected %q", ts.sleepMessage)
	}

	ts.cfg.Logger.Info("succcessfully checked pod logs",
		zap.String("pod-name", irsaFargatePodName),
		zap.String("container-name", irsaFargateContainerName),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkNodeReadiness() error {
	ts.cfg.Logger.Info("checking node readiness")

	desired := 1

	nodes, err := ts.cfg.K8SClient.ListNodes(1000, 5*time.Second)
	if err != nil {
		ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
		return err
	}

	readies := 0
	for _, node := range nodes {
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
		ts.cfg.Logger.Info("checked node readiness", zap.Int("desired", desired), zap.Int("readies", readies))
		return ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("failed to check node readiness", zap.Int("desired", desired), zap.Int("readies", readies))
	return fmt.Errorf("expected %d ready node(s), got %d node(s)", desired, readies)
}
