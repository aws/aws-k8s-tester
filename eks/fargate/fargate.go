// Package fargate implements tester for Fargate.
package fargate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/fargate/wait"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	aws_ecr "github.com/aws/aws-k8s-tester/pkg/aws/ecr"
	aws_iam "github.com/aws/aws-k8s-tester/pkg/aws/iam"
	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines "Fargate" configuration.
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
	cfg      Config
	ecrImage string
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFargate() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnFargate.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnFargate.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFargate.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if ts.cfg.EKSConfig.AddOnFargate.RepositoryName != "" {
		if ts.ecrImage, err = aws_ecr.Check(
			ts.cfg.Logger,
			ts.cfg.ECRAPI,
			ts.cfg.EKSConfig.AddOnFargate.RepositoryAccountID,
			ts.cfg.EKSConfig.AddOnFargate.RepositoryName,
			ts.cfg.EKSConfig.AddOnFargate.RepositoryImageTag,
		); err != nil {
			return err
		}
	}
	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnFargate.Namespace,
	); err != nil {
		return err
	}
	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createSecret(); err != nil {
		return err
	}
	if err = ts.createProfile(); err != nil {
		return err
	}
	if err = ts.createPod(); err != nil {
		return err
	}
	if err = ts.checkPod(); err != nil {
		return err
	}
	if err = ts.checkNode(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnFargate() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnFargate.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnFargate.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
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

	if err := ts.deleteRole(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate IAM Role (%v)", err))
	}
	ts.cfg.Logger.Info("wait after deleting IAM Role")
	time.Sleep(20 * time.Second)

	if err := ts.deleteSecret(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate Secret (%v)", err))
	}

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnFargate.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Fargate namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnFargate.Created = false
	return ts.cfg.EKSConfig.Sync()
}

// TemplateRole is the CloudFormation template for EKS Fargate role.
const TemplateRole = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS Cluster Fargate Role'

Parameters:

  FargateRoleName:
    Type: String
    Description: The name of the Fargate role

  FargateRoleServicePrincipals:
    Type: CommaDelimitedList
    Default: 'eks.amazonaws.com,eks-fargate-pods.amazonaws.com'
    Description: EKS Fargate Role Service Principals

  FargateRoleManagedPolicyARNs:
    Type: CommaDelimitedList
    Default: 'arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy'
    Description: EKS Fargate policy ARNs

Resources:

  FargateRole:
    Type: AWS::IAM::Role
    Properties:
      RoleName: !Ref FargateRoleName
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service: !Ref FargateRoleServicePrincipals
          Action:
          - sts:AssumeRole
      ManagedPolicyArns: !Ref FargateRoleManagedPolicyARNs
      Path: /

Outputs:

  FargateRoleARN:
    Value: !GetAtt FargateRole.Arn
    Description: The Fargate role ARN

`

func (ts *tester) createRole() error {
	if !ts.cfg.EKSConfig.AddOnFargate.RoleCreate {
		ts.cfg.Logger.Info("EKSConfig.AddOnFargate.RoleCreate false; skipping creation")
		return aws_iam.Validate(
			ts.cfg.Logger,
			ts.cfg.IAMAPI,
			ts.cfg.EKSConfig.AddOnFargate.RoleName,
			[]string{
				"eks.amazonaws.com",
				"eks-fargate-pods.amazonaws.com",
			},
			[]string{
				"arn:aws:iam::aws:policy/AmazonEKSFargatePodExecutionRolePolicy",
			},
		)
	}
	if ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID != "" &&
		ts.cfg.EKSConfig.AddOnFargate.RoleARN != "" {
		ts.cfg.Logger.Info("role already created; no need to create a new one")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnFargate.RoleName == "" {
		return errors.New("cannot create a cluster role with an empty AddOnFargate.RoleName")
	}

	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackYAMLPath, []byte(TemplateRole), 0400); err != nil {
		return err
	}
	if err := aws_s3.Upload(
		ts.cfg.Logger,
		ts.cfg.S3API,
		ts.cfg.EKSConfig.S3BucketName,
		ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackYAMLS3Key,
		ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackYAMLPath,
	); err != nil {
		return err
	}
	ts.cfg.Logger.Info("creating a new Fargate role using CFN",
		zap.String("role-name", ts.cfg.EKSConfig.AddOnFargate.RoleName),
		zap.String("role-cfn-file-path", ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackYAMLPath),
	)
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(ts.cfg.EKSConfig.AddOnFargate.RoleName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(TemplateRole),
		Tags: cfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("FargateRoleName"),
				ParameterValue: aws.String(ts.cfg.EKSConfig.AddOnFargate.RoleName),
			},
		},
	}
	if len(ts.cfg.EKSConfig.AddOnFargate.RoleServicePrincipals) > 0 {
		ts.cfg.Logger.Info("creating a new Fargate role with custom service principals",
			zap.Strings("service-principals", ts.cfg.EKSConfig.AddOnFargate.RoleServicePrincipals),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("FargateRoleServicePrincipals"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnFargate.RoleServicePrincipals, ",")),
		})
	}
	if len(ts.cfg.EKSConfig.AddOnFargate.RoleManagedPolicyARNs) > 0 {
		ts.cfg.Logger.Info("creating a new Fargate role with custom managed role policies",
			zap.Strings("policy-arns", ts.cfg.EKSConfig.AddOnFargate.RoleManagedPolicyARNs),
		)
		stackInput.Parameters = append(stackInput.Parameters, &cloudformation.Parameter{
			ParameterKey:   aws.String("FargateRoleManagedPolicyARNs"),
			ParameterValue: aws.String(strings.Join(ts.cfg.EKSConfig.AddOnFargate.RoleManagedPolicyARNs, ",")),
		})
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID = aws.StringValue(stackOutput.StackId)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to create Fargate role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()

	for _, o := range st.Stack.Outputs {
		switch k := aws.StringValue(o.OutputKey); k {
		case "FargateRoleARN":
			ts.cfg.EKSConfig.AddOnFargate.RoleARN = aws.StringValue(o.OutputValue)
		default:
			return fmt.Errorf("unexpected OutputKey %q from %q", k, ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID)
		}
	}

	ts.cfg.Logger.Info("created a Fargate role",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID),
		zap.String("role-name", ts.cfg.EKSConfig.AddOnFargate.RoleName),
		zap.String("role-arn", ts.cfg.EKSConfig.AddOnFargate.RoleARN),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteRole() error {
	if ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID == "" {
		ts.cfg.Logger.Info("empty Fargate role CFN stack ID; no need to delete Fargate")
		return nil
	}

	ts.cfg.Logger.Info("deleting Fargate role CFN stack",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID),
	)
	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID),
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
		ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		time.Minute,
		10*time.Second,
	)
	var st cfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to delete Fargate role (%v)", st.Error))
			return st.Error
		}
	}
	cancel()
	ts.cfg.Logger.Info("deleted a Fargate role",
		zap.String("role-cfn-stack-id", ts.cfg.EKSConfig.AddOnFargate.RoleCFNStackID),
	)
	return ts.cfg.EKSConfig.Sync()
}

const secretReadTxt = "HELLO-WORLD-SECRET-IN-FARGATE"

func (ts *tester) createSecret() error {
	ts.cfg.Logger.Info("creating secret", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.SecretName))

	secret := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ts.cfg.EKSConfig.AddOnFargate.SecretName,
			Namespace: ts.cfg.EKSConfig.AddOnFargate.Namespace,
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{ts.cfg.EKSConfig.AddOnFargate.SecretName: []byte(secretReadTxt)},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Secrets(ts.cfg.EKSConfig.AddOnFargate.Namespace).
		Create(ctx, secret, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created secret", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.SecretName))
	return ts.cfg.EKSConfig.Sync()
}

var propagationBackground = metav1.DeletePropagationBackground

func (ts *tester) deleteSecret() error {
	ts.cfg.Logger.Info("deleting Secret", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.SecretName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		CoreV1().
		Secrets(ts.cfg.EKSConfig.AddOnFargate.Namespace).
		Delete(
			ctx,
			ts.cfg.EKSConfig.AddOnFargate.SecretName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete Secret %q (%v)", ts.cfg.EKSConfig.AddOnFargate.SecretName, err)
	}
	ts.cfg.Logger.Info("deleted Secret", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.SecretName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createProfile() error {
	if ts.cfg.EKSConfig.AddOnFargate.RoleARN == "" {
		return errors.New("empty AddOnFargate.RoleARN")
	}
	if len(ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs) == 0 {
		return errors.New("empty Parameters.PrivateSubnetIDs")
	}
	ts.cfg.Logger.Info("creating fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.ProfileName))

	req, _ := ts.cfg.EKSAPI.CreateFargateProfileRequest(&eks.CreateFargateProfileInput{
		ClusterName:         aws.String(ts.cfg.EKSConfig.Name),
		FargateProfileName:  aws.String(ts.cfg.EKSConfig.AddOnFargate.ProfileName),
		PodExecutionRoleArn: aws.String(ts.cfg.EKSConfig.AddOnFargate.RoleARN),
		Subnets:             aws.StringSlice(ts.cfg.EKSConfig.Parameters.PrivateSubnetIDs),
		Selectors: []*eks.FargateProfileSelector{
			{
				Namespace: aws.String(ts.cfg.EKSConfig.AddOnFargate.Namespace),
			},
		},
	})
	err := req.Send()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("sent create fargate profile request")

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	ch := wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnFargate.ProfileName,
		eks.FargateProfileStatusActive,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		err = sv.Error
	}
	cancel()
	if err != nil {
		return fmt.Errorf("failed to wait for fargate profile creation %v", err)
	}

	ts.cfg.Logger.Info("created fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.ProfileName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteProfile() error {
	ts.cfg.Logger.Info("deleting fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.ProfileName))

	var err error
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < time.Minute {
		_, err = ts.cfg.EKSAPI.DeleteFargateProfile(&eks.DeleteFargateProfileInput{
			ClusterName:        aws.String(ts.cfg.EKSConfig.Name),
			FargateProfileName: aws.String(ts.cfg.EKSConfig.AddOnFargate.ProfileName),
		})
		if err != nil && !wait.IsProfileDeleted(err) {
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
	ch := wait.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.EKSAPI,
		ts.cfg.EKSConfig.Name,
		ts.cfg.EKSConfig.AddOnFargate.ProfileName,
		wait.FargateProfileStatusDELETEDORNOTEXIST,
		10*time.Second,
		7*time.Second,
	)
	for sv := range ch {
		err = sv.Error
	}
	cancel()
	if err != nil {
		return fmt.Errorf("failed to wait for fargate profile deletion %v", err)
	}

	ts.cfg.Logger.Info("deleted fargate profile", zap.String("name", ts.cfg.EKSConfig.AddOnFargate.ProfileName))
	return ts.cfg.EKSConfig.Sync()
}

const (
	fargatePodName       = "fargate-pod"
	fargateContainerName = "fargate-container"
)

func (ts *tester) createPod() error {
	if err := ts.listPods(ts.cfg.EKSConfig.AddOnFargate.Namespace); err != nil {
		ts.cfg.Logger.Warn("listing pods failed", zap.Error(err))
	}

	image := "amazonlinux:latest"
	if ts.cfg.EKSConfig.AddOnFargate.RepositoryName != "" {
		image = ts.ecrImage
	}
	ts.cfg.Logger.Info("creating Fargate Pod", zap.String("image", image))

	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fargatePodName,
			Namespace: ts.cfg.EKSConfig.AddOnFargate.Namespace,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name:            fargateContainerName,
					Image:           image,
					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"/bin/sh",
						"-c",
					},
					Args: []string{
						fmt.Sprintf("cat /tmp/%s && sleep 10000", ts.cfg.EKSConfig.AddOnFargate.SecretName),
					},

					// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "secret-volume",
							MountPath: "/tmp",
							ReadOnly:  true,
						},
					},
				},
			},

			// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
			Volumes: []v1.Volume{
				{ // to read
					Name: "secret-volume",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: ts.cfg.EKSConfig.AddOnFargate.SecretName,
						},
					},
				},
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnFargate.Namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created Pod")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deletePod() error {
	ts.cfg.Logger.Info("deleting Pod", zap.String("name", fargatePodName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.
		K8SClient.KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnFargate.Namespace).
		Delete(
			ctx,
			fargatePodName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &propagationBackground,
			},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete Pod %q (%v)", fargatePodName, err)
	}
	ts.cfg.Logger.Info("deleted Pod", zap.String("name", fargatePodName))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) listPods(ns string) error {
	pods, err := ts.getPods(ns)
	if err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")
	for _, v := range pods.Items {
		fmt.Fprintf(ts.cfg.LogWriter, "%q Pod using client-go: %q\n", ns, v.Name)
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n")
	return nil
}

func (ts *tester) getPods(ns string) (*v1.PodList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	ps, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	cancel()
	return ps, err
}

func (ts *tester) checkPod() error {
	execArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnFargate.Namespace,
		"exec",
		"-it",
		fargatePodName,
		"--",
		"cat",
		fmt.Sprintf("/tmp/%s", ts.cfg.EKSConfig.AddOnFargate.SecretName),
	}
	execCmd := strings.Join(execArgs, " ")

	ts.cfg.Logger.Info("checking Pod exec",
		zap.String("container-name", fargateContainerName),
		zap.String("command", execCmd),
	)

	succeeded := false
	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("aborted")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, execArgs[0], execArgs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl exec' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", execCmd, out)

		if !strings.Contains(out, secretReadTxt) {
			ts.cfg.Logger.Warn("unexpected exec output", zap.String("output", out))
			time.Sleep(5 * time.Second)
			continue
		}

		succeeded = true
		ts.cfg.Logger.Info("successfully checked Pod exec",
			zap.String("pod-name", fargatePodName),
			zap.String("container-name", fargateContainerName),
		)
		break
	}
	if !succeeded {
		ts.cfg.EKSConfig.Sync()
		return errors.New("failed to find expected output from kubectl exec")
	}

	descArgsPods := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnFargate.Namespace,
		"describe",
		"pods/" + fargatePodName,
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	logArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnFargate.Namespace,
		"logs",
		"pods/" + fargatePodName,
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	ts.cfg.Logger.Info("checking Pod",
		zap.String("pod-name", fargatePodName),
		zap.String("container-name", fargateContainerName),
		zap.String("command-describe", descCmdPods),
		zap.String("command-logs", logsCmd),
	)

	succeeded = false
	retryStart, waitDur = time.Now(), 2*time.Minute
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("aborted")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, out)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", logsCmd, out)

		if !strings.Contains(out, secretReadTxt) {
			ts.cfg.Logger.Warn("unexpected logs output", zap.String("output", out))
			time.Sleep(5 * time.Second)
			continue
		}

		succeeded = true
		ts.cfg.Logger.Info("successfully checked Pod logs",
			zap.String("pod-name", fargatePodName),
			zap.String("container-name", fargateContainerName),
		)
		break
	}
	if !succeeded {
		// TODO: expected output not found, fail the whole tester
		ts.cfg.Logger.Warn("failed to find expected output from kubectl logs; fail!", zap.String("expected", secretReadTxt))
	}

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
			nodeName := node.GetName()
			labels := node.GetLabels()
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
