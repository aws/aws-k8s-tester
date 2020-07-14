// Package irsa implements tester for IAM Roles for Service Accounts (IRSA).
package irsa

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
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
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines "IAM Roles for Service Accounts (IRSA)" configuration.
// ref. https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	S3API     s3iface.S3API
	CFNAPI    cloudformationiface.CloudFormationAPI
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
	cfg               Config
	ecrImage          string
	deploymentCreated time.Time
	sleepMessage      string
	testBody          string
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
		ts.cfg.EKSConfig.AddOnIRSA.RepositoryRegion,
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
	if err = ts.createDeployment(); err != nil {
		return err
	}
	if err = ts.waitDeployment(); err != nil {
		return err
	}
	if err = ts.checkPodWebhook(); err != nil {
		return err
	}
	if err = ts.checkResults(); err != nil {
		return err
	}

	ts.cfg.EKSConfig.AddOnIRSA.DeploymentTook = time.Since(ts.deploymentCreated)
	ts.cfg.EKSConfig.AddOnIRSA.DeploymentTookString = ts.cfg.EKSConfig.AddOnIRSA.DeploymentTook.String()
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
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete IRSA namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnIRSA.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createS3Object() (err error) {
	if ts.cfg.EKSConfig.S3BucketName == "" {
		return errors.New("empty S3 bucket name for IRSA add-on")
	}
	ts.testBody = randutil.String(256)
	ts.sleepMessage = `SUCCESS IRSA TEST: SLEEPING WITH ` + randutil.String(32)
	return aws_s3.UploadBody(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnIRSA.S3Key,
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

// TemplateRole is the CloudFormation template for EKS IRSA role.
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster IRSA Role'

Parameters:

  RoleName:
    Type: String
    Description: The name of the IRSA role

  IssuerARN:
    Type: String
    Description: EKS IRSA Provider ARN

  Namespace:
    Type: String
    Description: The namespace for the IRSA role

  ServiceAccountName:
    Type: String
    Description: The ServiceAccount name for the IRSA role

Resources:

  IRSARole:
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

  IRSARoleARN:
    Description: The IRSA role ARN
    Value: !GetAtt IRSARole.Arn

`

type irsaTemplate struct {
	IRSAIssuerHostPath string
	S3BucketName       string
	ClusterName        string
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
		S3BucketName:       ts.cfg.EKSConfig.S3BucketName,
		ClusterName:        ts.cfg.EKSConfig.Name,
	}); err != nil {
		return err
	}

	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLPath, buf.Bytes(), 0400); err != nil {
		return err
	}
	if err := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLS3Key,
		ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLPath,
	); err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating a new IRSA role using CFN",
		zap.String("role-name", ts.cfg.EKSConfig.AddOnIRSA.RoleName),
		zap.String("role-cfn-file-path", ts.cfg.EKSConfig.AddOnIRSA.RoleCFNStackYAMLPath),
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
			"User":                   user.Get(),
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("RoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSA.RoleName),
			},
			{
				ParameterKey:   aws.String("IssuerARN"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.Status.ClusterOIDCIssuerARN),
			},
			{
				ParameterKey:   aws.String("Namespace"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnIRSA.Namespace),
			},
			{
				ParameterKey:   aws.String("ServiceAccountName"),
				ParameterValue: aws.String(irsaServiceAccountName),
			},
		},
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
		ts.cfg.LogWriter,
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
		ts.cfg.LogWriter,
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
	irsaAppName            = "irsa-app"
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
// Do not download to the same file paths.
// e.g. download failed: s3://aws-k8s-tester-eks-s3-bucket/eks-2020062621-misty8up27dz/irsa-s3-key to var/log/output-configmap.log [Errno 16] Device or resource busy: '/var/log/output-configmap.log.75Caa245' -> '/var/log/output-configmap.log'
// ${HOSTNAME} is same as Pod name, writes "/var/log/[POD_NAME].s3.output"
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
# aws s3 cp s3://{{ .S3BucketName }}/{{ .S3Key }} /var/log/$HOSTNAME.s3.output;

printf "\n's3-utils cp':\n"
/s3-utils cp --log-level info --partition {{.Partition}} --region {{.Region}} --s3-bucket {{ .S3BucketName }} --s3-key {{ .S3Key }} --local-path /var/log/$HOSTNAME.s3.output --timeout 10s;

printf "\n"
echo {{ .S3Key }} contents:
cat /var/log/$HOSTNAME.s3.output;
printf "\n\nSUCCESS IRSA TEST: S3 FILE DOWNLOADED!\n\n"

printf "\n'sts-utils get-caller-identity' expected role ARN:\n"
/sts-utils get-caller-identity --partition {{.Partition}} --region {{.Region}} --match-contain-role-arn {{ .RoleName }}
printf "\nSUCCESS IRSA TEST: CALLER_ROLE_ARN FOUND!\n\n"

printf "\n{{ .SleepMessage }}\n\n"
sleep 86400


printf "\nSUCCESS IRSA TEST: EXITING...\n\n"
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
	ts.cfg.Logger.Info("creating configmap", zap.String("name", irsaConfigMapName))

	tpl := template.Must(template.New("TemplateConfigMap").Parse(TemplateConfigMap))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, configMapTemplate{
		Partition: ts.cfg.EKSConfig.Partition,
		Region:    ts.cfg.EKSConfig.Region,

		// e.g.
		// created role ARN:    arn:aws:iam::607362164682:role/eks-2020071200-galaxyzejwho-add-on-irsa-role
		// sts caller role ARN: arn:aws:sts::607362164682:assumed-role/eks-2020071200-galaxyzejwho-add-on-irsa-role/botocore-session-1594541343
		RoleName: ts.cfg.EKSConfig.AddOnIRSA.RoleName,

		S3BucketName: ts.cfg.EKSConfig.S3BucketName,
		S3Key:        ts.cfg.EKSConfig.AddOnIRSA.S3Key,
		SleepMessage: ts.sleepMessage,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()
	fmt.Fprintf(ts.cfg.LogWriter, "\nAddOnIRSA ConfigMap:\n%s\n\n", tplTxt)

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

	ts.cfg.Logger.Info("created configmap", zap.String("name", irsaConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMaps() error {
	ts.cfg.Logger.Info("deleting configmap", zap.String("name", irsaConfigMapName))
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

	ts.cfg.Logger.Info("deleted configmap", zap.String("name", irsaConfigMapName))
	return ts.cfg.EKSConfig.Sync()
}

// TemplateDeploymentScript is the script to run in Deployment.
const TemplateDeploymentScript = `printf '\n\nexecuting...\n\n'; /opt/{{ .ConfigMapScriptFileName }};`

type deploymentScriptTemplate struct {
	ConfigMapScriptFileName string
}

func (ts *tester) createDeployment() error {
	ts.cfg.Logger.Info("creating IRSA Deployment")

	tpl := template.Must(template.New("TemplateDeploymentScript").Parse(TemplateDeploymentScript))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, deploymentScriptTemplate{
		ConfigMapScriptFileName: irsaConfigMapFileName,
	}); err != nil {
		return err
	}
	tplTxt := buf.String()

	ts.cfg.Logger.Info("creating IRSA Deployment", zap.String("image", ts.ecrImage))
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
						"app.kubernetes.io/name": irsaAppName,
					},
				},
				Spec: apps_v1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": irsaAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": irsaAppName,
							},
						},
						Spec: v1.PodSpec{
							ServiceAccountName: irsaServiceAccountName,

							// invalid: spec.template.spec.restartPolicy: Unsupported value: \"OnFailure\": supported values: \"Always\")"
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            irsaAppName,
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
								// "AMIType": ec2config.AMITypeAL2X8664,
								// do not deploy in fake nodes, obviously
								"NodeType": "regular",
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
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete IRSA Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted IRSA Deployment", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() (err error) {
	timeout := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = k8s_client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		20*time.Second,
		ts.cfg.EKSConfig.AddOnIRSA.Namespace,
		irsaDeploymentName,
		ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas,
		k8s_client.WithQueryFunc(func() {
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
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

			getArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=" + ts.cfg.EKSConfig.AddOnIRSA.Namespace,
				"get",
				"pods",
			}
			getCmd := strings.Join(getArgs, " ")
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			cmdOutput, err := exec.New().CommandContext(ctx, getArgs[0], getArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get pods' failed", zap.Error(err))
			}
			out = string(cmdOutput)
			fmt.Fprintf(ts.cfg.LogWriter, "\"%s\" output:\n\n%s\n\n", getCmd, out)

			pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnIRSA.Namespace, 3000, 3*time.Second)
			if err != nil {
				ts.cfg.Logger.Warn("failed to list Pod", zap.Bool("retriable-error", k8s_client.IsRetryableAPIError(err)), zap.Error(err))
				return
			}
			if len(pods) == 0 {
				ts.cfg.Logger.Warn("got an empty list of Pod")
				return
			}
			for _, pod := range pods {
				ts.cfg.Logger.Info("pod",
					zap.String("pod-name", pod.Name),
					zap.String("pod-status-phase", fmt.Sprintf("%v", pod.Status.Phase)),
				)
				if pod.Status.Phase == v1.PodFailed {
					logsArgs := []string{
						ts.cfg.EKSConfig.KubectlPath,
						"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
						"--namespace=" + ts.cfg.EKSConfig.AddOnIRSA.Namespace,
						"logs",
						fmt.Sprintf("pod/%s", pod.Name),
						"--timestamps",
					}
					logsCmd := strings.Join(logsArgs, " ")
					ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
					cmdOutput, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
					cancel()
					if err != nil {
						ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
					}
					out = string(cmdOutput)
					fmt.Fprintf(ts.cfg.LogWriter, "\"%s\" output:\n\n%s\n\n", logsCmd, out)
				}
			}
		}),
	)
	cancel()
	return err
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

		pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnIRSA.Namespace, 150, 5*time.Second)
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

	ts.cfg.Logger.Info("checked IRSA Pod spec for webhook")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkResults() (err error) {
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
		if err = ts.checkLogs(); err != nil {
			ts.cfg.Logger.Warn("failed to check logs", zap.Error(err))
			continue
		}
		ready = true
		break
	}
	if !ready {
		return errors.New("failed to check results for IRSA Pod")
	}
	ts.cfg.Logger.Info("checked results")
	return ts.cfg.EKSConfig.Sync()
}

// 1. check pod logs if configmap run succeeds
// 2. check node "/var/log" for expected outputs
func (ts *tester) checkLogs() error {
	expects := int(ts.cfg.EKSConfig.AddOnIRSA.DeploymentReplicas)
	ts.cfg.Logger.Info("checking logs from IRSA pods and nodes", zap.Int("expects", expects))
	pods, err := ts.cfg.K8SClient.ListPods(ts.cfg.EKSConfig.AddOnIRSA.Namespace, 150, 5*time.Second)
	if err != nil {
		return err
	}

	os.RemoveAll(ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath)
	f, err := os.OpenFile(ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(ts.cfg.EKSConfig.AddOnIRSA.DeploymentResultPath)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	success := 0
	for _, pod := range pods {
		podName := pod.Name
		nodeName := pod.Spec.NodeName
		ts.cfg.Logger.Info("pod",
			zap.String("pod-name", podName),
			zap.String("node-name", nodeName),
		)
		if !strings.HasPrefix(pod.Name, irsaDeploymentName) {
			continue
		}

		fmt.Fprintf(ts.cfg.LogWriter, "\n*********************************\n")
		descArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnIRSA.Namespace,
			"describe",
			"pod",
			podName,
		}
		descCmd := strings.Join(descArgs, " ")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		runOutput, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(runOutput))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "'%s' output:\n\n%s\n\n", descCmd, output)

		logsArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=" + ts.cfg.EKSConfig.AddOnIRSA.Namespace,
			"logs",
			"pod/" + podName,
			"--timestamps",
		}
		logsCmd := strings.Join(logsArgs, " ")
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		runOutput, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		output = strings.TrimSpace(string(runOutput))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl logs", zap.Error(err))
		}
		cur, ok := ts.cfg.EKSConfig.Status.PrivateDNSToNodeInfo[nodeName]
		if !ok {
			return fmt.Errorf("node %q unknown", nodeName)
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output from pod %q in node %q, node group name %q, node group ami type %q, public IP %q, public DNS name %q, user name %q (expects %q):\n\n%s\n\n", logsCmd, pod.Name, nodeName, cur.NodeGroupName, cur.AMIType, cur.PublicIP, cur.PublicDNSName, cur.UserName, ts.sleepMessage, output)
		if !strings.Contains(output, ts.sleepMessage) {
			fmt.Fprintf(ts.cfg.LogWriter, "\n*********************************\n")
			continue
		}
		if _, err = f.WriteString(fmt.Sprintf("'%s' from pod %q in node %q, node group name %q, node group ami type %q, public IP %q, public DNS name %q, user name %q:\n\n%s\n\n", logsCmd, pod.Name, nodeName, cur.NodeGroupName, cur.AMIType, cur.PublicIP, cur.PublicDNSName, cur.UserName, output)); err != nil {
			ts.cfg.Logger.Warn("failed to write", zap.Error(err))
			fmt.Fprintf(ts.cfg.LogWriter, "\n*********************************\n")
			continue
		}
		ts.cfg.Logger.Info("checked pod logs, found matching sleep message", zap.String("pod-name", podName))
		fmt.Fprintf(ts.cfg.LogWriter, "\n*********************************\n")
		success++
	}
	if success < expects {
		ts.cfg.Logger.Warn("not enough successful pod",
			zap.Int("success", success),
			zap.Int("expects", expects),
		)
		if success == 0 {
			return errors.New("no IRSA Pod is ready")
		}
		// TODO: require success >= expects
	}
	ts.cfg.Logger.Info("checked logs from IRSA pods and nodes",
		zap.Int("success", success),
		zap.Int("expects", expects),
	)
	return ts.cfg.EKSConfig.Sync()
}
