// Package irsa implements tester for IAM Roles for Service Accounts (IRSA).
package irsa

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/ssh"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines "IAM Roles for Service Accounts (IRSA)" configuration.
// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	CFNAPI    cloudformationiface.CloudFormationAPI
	IAMAPI    iamiface.IAMAPI
	S3API     s3iface.S3API
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
	cfg               Config
	ecrImage          string
	deploymentCreated time.Time
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnIRSA() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnIRSA.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnIRSA.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnIRSA.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.ecrImage, err = aws_ecr.Check(
		ts.cfg.Logger,
		ts.cfg.ECRAPI,
		ts.cfg.EKSConfig.AddOnIRSA.RepositoryAccountID,
		ts.cfg.EKSConfig.AddOnIRSA.RepositoryName,
		ts.cfg.EKSConfig.AddOnIRSA.RepositoryImageTag,
	); err != nil {
		return err
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnIRSA.Namespace,
	); err != nil {
		return err
	}
	if err = ts.createS3(); err != nil {
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
	if err = ts.createConfigMaps(); err != nil {
		return err
	}
	if err = ts.createDeployment(); err != nil {
		return err
	}
	if err = ts.checkPods(); err != nil {
		return err
	}
	if err = ts.waitDeployment(); err != nil {
		return err
	}
	if err = ts.waitOutputLogs(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnIRSA() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnIRSA.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnIRSA.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Deployments (%v)", err))
	}
	time.Sleep(2 * time.Minute)

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
		errs = append(errs, fmt.Sprintf("failed to delete IAM Role (%v)", err))
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
		ts.cfg.EKSConfig.AddOnIRSA.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete IRSA namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnIRSA.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createS3() (err error) {
	if ts.cfg.EKSConfig.S3BucketName == "" {
		return errors.New("empty S3 bucket name for IRSA add-on")
	}
	return aws_s3.UploadBody(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnIRSA.S3Key,
		bytes.NewReader(randutil.Bytes(1024)),
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

// TemplateRole is the CloudFormation template for EKS IRSA role.
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster IRSA Role'

Parameters:

  IRSARoleName:
    Type: String
    Description: The name of the IRSA role

  IRSAIssuerARN:
    Type: String
    Description: EKS IRSA Provider ARN

  IRSANamespace:
    Type: String
    Description: The namespace for the IRSA role

  IRSAServiceAccountName:
    Type: String
    Description: The ServiceAccount name for the IRSA role

  IRSARoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess'
    Description: EKS IRSA policy ARNs

Resources:

  IRSARole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref IRSARoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Effect: Allow
            Principal:
              Federated: !Ref IRSAIssuerARN
            Action:
            - sts:AssumeRoleWithWebIdentity
            Condition:
              StringEquals:
                {{ .IRSAIssuerHostPath }}:sub: !Join [':', ['system:serviceaccount', !Ref IRSANamespace, !Ref IRSAServiceAccountName]]
      ManagedPolicyArns: !Ref IRSARoleManagedPolicyARNs

Outputs:

  IRSARoleARN:
    Description: The IRSA role ARN
    Value: !GetAtt IRSARole.Arn

`

type irsaTemplate struct {
	IRSAIssuerHostPath string
}

func (ts *tester) createRole() error {
	if ts.cfg.EKSConfig.AddOnIRSA.RoleName == "" {
		return errors.New("empty AddOnIRSA.RoleName")
	}
	if ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID != "" ||
		ts.cfg.EKSConfig.AddOnIRSA.RoleARN != "" {
		ts.cfg.Logger.Info("non-empty roleARN given; no need to create a new one")
		return nil
	}

	tpl := template.Must(template.New("TemplateRole").Parse(TemplateRole))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, irsaTemplate{
		IRSAIssuerHostPath: ts.cfg.EKSConfig.Status.ClusterOIDCIssuerHostPath,
	}); err != nil {
		return err
	}

	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLFilePath, buf.Bytes(), 0400); err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating a new IRSA role using CFN",
		zap.String("role-name", ts.cfg.EKSConfig.AddOnIRSA.RoleName),
		zap.String("role-cfn-file-path", ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLFilePath),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnIRSA.RoleName),
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
				ParameterKey:   aws.String("IRSARoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSA.RoleName),
			},
			{
				ParameterKey:   aws.String("IRSAIssuerARN"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
			},
			{
				ParameterKey:   aws.String("IRSANamespace"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSA.Namespace),
			},
			{
				ParameterKey:   aws.String("IRSAServiceAccountName"),
				ParameterValue: aws.String(irsaServiceAccountName),
			},
		},
	}
	if len(ts.cfg.EKSConfig.AddOnIRSA.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new IRSA role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.AddOnIRSA.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("IRSARoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnIRSA.RoleManagedPolicyARNs, ",")),
		})
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create IRSA role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "IRSARoleARN":
			ts.cfg.EKSConfig.AddOnIRSA.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a IRSA role",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID),
		zap.String("role-name", ts.cfg.EKSConfig.AddOnIRSA.RoleName),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnIRSA.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteRole() error {
	if ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty IRSA role CFN stack ID; no need to delete IRSA")
		return nil
	}

	ts.cfg.Logger.Info("deleting IRSA role CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID),
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
		ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID,
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
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}

const (
	irsaServiceAccountName = "irsa-service-account"
	irsaConfigMapName      = "irsa-configmap"
	irsaConfigMapFileName  = "irsa-configmap.bash"
	irsaDeploymentName     = "irsa-deployment"
)

func (ts *tester) createServiceAccount() error {
	ts.cfg.Logger.Info("creating service account", zap.String("name", irsaServiceAccountName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Create(
			ctx,
			&v1.ServiceAccount{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      irsaServiceAccountName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSA.Namespace,
					Labels: map[string]string{
						"name": irsaServiceAccountName,
					},
					Annotations: map[string]string{
						"eks.amazonaws.com/role-arn": ts.cfg.EKSConfig.AddOnIRSA.RoleARN,
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created service account", zap.String("name", irsaServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteServiceAccount() error {
	ts.cfg.Logger.Info("deleting service account", zap.String("name", irsaServiceAccountName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ServiceAccounts(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Delete(
			ctx,
			irsaServiceAccountName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted service account", zap.String("name", irsaServiceAccountName))
	return ts.cfg.EKSConfig.Sync()
}

// TemplateConfigMap is the IRSA config map.
const TemplateConfigMap = `
#!/usr/bin/env bash
set -e
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
printf "\nSUCCESS IRSA TEST: CALLER_ROLE_ARN FOUND!\n\n"
aws s3 cp s3://{{ .S3BucketName }}/{{ .S3Key }} /tmp/{{ .S3Key }}
printf "\n"
echo {{ .S3Key }} contents:
cat /tmp/{{ .S3Key }}
printf "\nSUCCESS IRSA TEST: S3 FILE DOWNLOADED!\n\n"
printf "\n{{ .SleepMessage }}\n\n"
sleep 86400
printf "\nSUCCESS IRSA TEST: EXITING...\n\n"
`

const sleepMsg = `SUCCESS IRSA TEST: SLEEPING...`

type configMapTemplate struct {
	RoleName     string
	S3BucketName string
	S3Key        string
	SleepMessage string
}

func (ts *tester) createConfigMaps() error {
	ts.cfg.Logger.Info("creating config maps", zap.String("name", irsaConfigMapName))

	tpl := template.Must(template.New("TemplateConfigMap").Parse(TemplateConfigMap))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, configMapTemplate{
		RoleName:     ts.cfg.EKSConfig.AddOnIRSA.RoleName,
		S3BucketName: ts.cfg.EKSConfig.S3BucketName,
		S3Key:        ts.cfg.EKSConfig.AddOnIRSA.S3Key,
		SleepMessage: sleepMsg,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      irsaConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSA.Namespace,
					Labels: map[string]string{
						"name": irsaConfigMapName,
					},
				},
				Data: map[string]string{
					irsaConfigMapFileName: tplTxt,
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created config maps", zap.String("name", irsaConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMaps() error {
	ts.cfg.Logger.Info("deleting config maps", zap.String("name", irsaConfigMapName))
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Delete(
			ctx,
			irsaConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("deleted config maps", zap.String("name", irsaConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

const outputFilePath = "/var/log/output-configmap.log"

// TemplateDeploymentScript is the script to run in Deployment.
const TemplateDeploymentScript = `printf '\n\n/opt/{{ .ConfigMapScriptFileName }}:\n' >> {{ .OutputFilePath }}; cat /opt/{{ .ConfigMapScriptFileName }} >>{{ .OutputFilePath }};  printf '\n\nexecuting...\n\n' >> {{ .OutputFilePath }}; /opt/{{ .ConfigMapScriptFileName }} 1>>{{ .OutputFilePath }} 2>>{{ .OutputFilePath }};`

type deploymentScriptTemplate struct {
	ConfigMapScriptFileName string
	OutputFilePath          string
}

func (ts *tester) createDeployment() error {
	ts.cfg.Logger.Info("creating IRSA Deployment")

	tpl := template.Must(template.New("TemplateDeploymentScript").Parse(TemplateDeploymentScript))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, deploymentScriptTemplate{
		ConfigMapScriptFileName: irsaConfigMapFileName,
		OutputFilePath:          outputFilePath,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ts.cfg.Logger.Info("creating IRSA Deployment", zap.String("image", ts.ecrImage))
	fileOrCreate := v1.HostPathFileOrCreate
	dirOrCreate := v1.HostPathDirectoryOrCreate
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Create(
			ctx,
			&apps_v1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      irsaDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnIRSA.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": irsaDeploymentName,
					},
				},
				Spec: apps_v1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": irsaDeploymentName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": irsaDeploymentName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: irsaServiceAccountName,

							// invalid: spec.template.spec.restartPolicy: Unsupported value: \"OnFailure\": supported values: \"Always\")"
							RestartPolicy: v1.RestartPolicyAlways,

							Containers: []v1.Container{
								{
									Name:            irsaDeploymentName,
									Image:           ts.ecrImage,
									ImagePullPolicy: v1.PullIfNotPresent,

									Command: []string{
										"sh",
										"-c",
										tplTxt,
									},

									// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
									VolumeMounts: []v1.VolumeMount{
										{ // to execute
											Name:      irsaConfigMapName,
											MountPath: "/opt",
										},
										{ // to write
											Name:      "output-file",
											MountPath: outputFilePath,
										},
										{ // to write
											Name:      "varlog",
											MountPath: "/var/log",
											ReadOnly:  false,
										},
									},
								},
							},

							// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
							Volumes: []v1.Volume{
								{ // to execute
									Name: irsaConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: irsaConfigMapName,
											},
											DefaultMode: aws.Int32(0777),
										},
									},
								},
								{ // to write
									Name: "output-file",
									VolumeSource: v1.VolumeSource{
										HostPath: &v1.HostPathVolumeSource{
											Path: outputFilePath,
											Type: &fileOrCreate,
										},
									},
								},
								{ // to write
									Name: "varlog",
									VolumeSource: v1.VolumeSource{
										HostPath: &v1.HostPathVolumeSource{
											Path: "/var/log",
											Type: &dirOrCreate,
										},
									},
								},
							},

							NodeSelector: map[string]string{
								// cannot fetch results from Bottlerocket
								"AMIType": ec2config.AMITypeAL2X8664,
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create IRSA Deployment (%v)", err)
	}

	ts.deploymentCreated = time.Now()
	ts.cfg.Logger.Info("created IRSA Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting IRSA Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
		Delete(
			ctx,
			irsaDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete IRSA Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted IRSA Deployment", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

// https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
func (ts *tester) checkPods() error {
	ts.cfg.Logger.Info("waiting for IRSA Pods")

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

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		presp, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
			List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get IRSA Pod (%v)", err)
		}
		ts.cfg.Logger.Info("listed Pods", zap.Int("items", len(presp.Items)))

		for _, pod := range presp.Items {
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
						if env.Value != ts.cfg.EKSConfig.AddOnIRSA.RoleARN {
							return fmt.Errorf("%q expected %q, got %q", env.Name, ts.cfg.EKSConfig.AddOnIRSA.RoleARN, env.Value)
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

	ts.cfg.Logger.Info("waited for IRSA Pods")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for IRSA Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnIRSA.Namespace,
		"describe",
		"deployment",
		irsaDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	initialWait := 2*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)*3*time.Second
	if initialWait > 10*time.Minute {
		initialWait = 10 * time.Minute
	}
	ts.cfg.Logger.Info("initial waiting", zap.Duration("duration", initialWait))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("check aborted")
	case <-time.After(initialWait):
	}
	ts.cfg.Logger.Info("initial waited", zap.Duration("duration", initialWait))

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)*3*time.Second
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(time.Minute):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnIRSA.Namespace).
			Get(ctx, irsaDeploymentName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == apps_v1.DeploymentAvailable {
				available = true
				break
			}
		}
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for IRSA Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitOutputLogs() error {
	expects := int(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)
	ts.cfg.Logger.Info("waiting for IRSA output logs",
		zap.String("path", outputFilePath),
		zap.Int("expects", expects),
	)

	waitDur := 2*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)*10*time.Second
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(20 * time.Second):
		}

		cnt, err := ts.countSuccess(expects)
		if err != nil {
			ts.cfg.Logger.Warn("failed to count from remotes", zap.Int("count", cnt), zap.Error(err))
		} else {
			ts.cfg.Logger.Info("counting success",
				zap.Int("expects", expects),
				zap.Int("current", cnt),
			)
		}
		if cnt >= expects {
			ts.cfg.EKSConfig.AddOnIRSA.DeploymentTook = time.Since(ts.deploymentCreated)
			ts.cfg.EKSConfig.AddOnIRSA.DeploymentTookString = ts.cfg.EKSConfig.AddOnIRSA.DeploymentTook.String()
			ts.cfg.EKSConfig.Sync()
			break
		}
	}

	ts.cfg.Logger.Info("waited for IRSA output logs")
	return ts.cfg.EKSConfig.Sync()
}

// TODO: only SSH into the one with IRSA deployment Pod
func (ts *tester) countSuccess(expects int) (int, error) {
	sshOpt := ssh.WithVerbose(ts.cfg.EKSConfig.LogLevel == "debug")
	rateLimiter := rate.NewLimiter(rate.Limit(50), 10)
	total := 0
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		for name, nodeGroup := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			if nodeGroup.AMIType == ec2config.AMITypeBottleRocketCPU {
				ts.cfg.Logger.Warn("skipping bottlerocket log fetch", zap.String("ng-name", name))
				continue
			}
			ts.cfg.Logger.Info("fetching outputs from node group",
				zap.String("ng-name", name),
				zap.String("ami-type", nodeGroup.AMIType),
				zap.Int("nodes", len(nodeGroup.Instances)),
			)

			for instID, iv := range nodeGroup.Instances {
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting fetcher")
					return 0, nil
				default:
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before SSH into the machine",
						zap.String("instance-id", instID),
					)
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter",
						zap.Error(werr),
					)
				}

				ts.cfg.Logger.Debug("fetching output", zap.String("instance-id", instID))
				sh, err := ssh.New(ssh.Config{
					Logger:        ts.cfg.Logger,
					KeyPath:       ts.cfg.EKSConfig.RemoteAccessPrivateKeyPath,
					PublicIP:      iv.PublicIP,
					PublicDNSName: iv.PublicDNSName,
					UserName:      iv.RemoteAccessUserName,
				})
				if err != nil {
					ts.cfg.Logger.Warn("failed to create SSH", zap.Error(err))
					continue
				}
				if err = sh.Connect(); err != nil {
					ts.cfg.Logger.Warn("failed to connect to SSH", zap.Error(err))
					sh.Close()
					continue
				}
				catCmd := "sudo cat " + outputFilePath
				out, err := sh.Run(catCmd, sshOpt)
				if err != nil {
					ts.cfg.Logger.Warn("failed to run SSH command", zap.Error(err))
					sh.Close()
					continue
				}
				sh.Close()

				total += strings.Count(string(out), sleepMsg)
				if total >= expects {
					break
				}
			}
		}
	}
	if total < expects &&
		ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() && ts.cfg.EKSConfig.AddOnManagedNodeGroups.FetchLogs {
		for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			if cur.AMIType == ec2config.AMITypeBottleRocketCPU {
				ts.cfg.Logger.Warn("skipping bottlerocket log fetch", zap.String("mng-name", mngName))
				continue
			}
			ts.cfg.Logger.Info("fetching outputs from managed node group",
				zap.String("mng-name", mngName),
				zap.Int("nodes", len(cur.Instances)),
			)

			for instID, iv := range cur.Instances {
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting fetcher")
					return 0, nil
				default:
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before SSH into the machine",
						zap.String("instance-id", instID),
					)
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter",
						zap.Error(werr),
					)
				}

				ts.cfg.Logger.Debug("fetching output", zap.String("instance-id", instID))
				sh, err := ssh.New(ssh.Config{
					Logger:        ts.cfg.Logger,
					KeyPath:       ts.cfg.EKSConfig.RemoteAccessPrivateKeyPath,
					PublicIP:      iv.PublicIP,
					PublicDNSName: iv.PublicDNSName,
					UserName:      iv.RemoteAccessUserName,
				})
				if err != nil {
					ts.cfg.Logger.Warn("failed to create SSH", zap.Error(err))
					continue
				}
				if err = sh.Connect(); err != nil {
					ts.cfg.Logger.Warn("failed to connect to SSH", zap.Error(err))
					sh.Close()
					continue
				}
				catCmd := "sudo cat " + outputFilePath
				out, err := sh.Run(catCmd, sshOpt)
				if err != nil {
					ts.cfg.Logger.Warn("failed to run SSH command", zap.Error(err))
					sh.Close()
					continue
				}
				sh.Close()

				total += strings.Count(string(out), sleepMsg)
			}
		}
	}
	return total, ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnIRSA() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnIRSA.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	sfx := filepath.Base(outputFilePath)
	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && ts.cfg.EKSConfig.AddOnNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from ngs")
		for name, v := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			if v.AMIType == ec2config.AMITypeBottleRocketCPU {
				ts.cfg.Logger.Warn("skipping bottlerocket log fetch", zap.String("ng-name", name))
				continue
			}
			for _, fpaths := range v.Logs {
				for _, fpath := range fpaths {
					if !strings.HasSuffix(fpath, sfx) {
						continue
					}
					if _, err = f.Write([]byte(fmt.Sprintf("%q contents:\n", fpath))); err != nil {
						return err
					}
					d, err := ioutil.ReadFile(fpath)
					if err != nil {
						ts.cfg.Logger.Warn("failed to read file", zap.Error(err))
						return err
					}
					if _, err = f.Write(d); err != nil {
						ts.cfg.Logger.Warn("failed to write a file", zap.Error(err))
						return err
					}
					if _, err = f.Write([]byte("\n\n\n")); err != nil {
						return err
					}
				}
			}
		}
	}
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() && ts.cfg.EKSConfig.AddOnManagedNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("fetching logs from mngs")
		for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			if cur.AMIType == ec2config.AMITypeBottleRocketCPU {
				ts.cfg.Logger.Warn("skipping bottlerocket log fetch", zap.String("mng-name", mngName))
				continue
			}
			for _, fpaths := range cur.Logs {
				for _, fpath := range fpaths {
					if !strings.HasSuffix(fpath, sfx) {
						continue
					}
					if _, err = f.Write([]byte(fmt.Sprintf("%q contents:\n", fpath))); err != nil {
						return err
					}
					d, err := ioutil.ReadFile(fpath)
					if err != nil {
						ts.cfg.Logger.Warn("failed to read file", zap.Error(err))
						return err
					}
					if _, err = f.Write(d); err != nil {
						ts.cfg.Logger.Warn("failed to write a file", zap.Error(err))
						return err
					}
					if _, err = f.Write([]byte("\n\n\n")); err != nil {
						return err
					}
				}
			}
		}
	}

	ts.cfg.Logger.Info("aggregated results from Deployments", zap.String("result-path", ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath))
	return ts.cfg.EKSConfig.Sync()
}
