// Package appmesh implements App Mesh add-on.
package appmesh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/helm"
	"github.com/aws/aws-k8s-tester/eksconfig"
	awscfn "github.com/aws/aws-k8s-tester/pkg/aws/cloudformation"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"go.uber.org/zap"
)

// Config defines AppMesh configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Sig    chan os.Signal

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	CFNAPI    cloudformationiface.CloudFormationAPI
}

// Tester defines AppMesh tester
type Tester interface {
	// Installs AppMesh controller/injector.
	Create() error
	// Clean up AppMesh controller/injector.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping create AddOnAppMesh")
		return nil
	}

	ts.cfg.EKSConfig.AddOnAppMesh.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnAppMesh.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnAppMesh.CreateTookString = ts.cfg.EKSConfig.AddOnAppMesh.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createPolicy(); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnAppMesh.Namespace); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createHelmController(); err != nil {
		return err
	}
	if err := ts.createHelmInjector(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping delete AddOnAppMesh")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnAppMesh.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnAppMesh.DeleteTookString = ts.cfg.EKSConfig.AddOnAppMesh.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteHelmInjector(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := ts.deleteHelmController(); err != nil {
		errs = append(errs, err.Error())
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete AppMesh namespace (%v)", err))
	}

	if err := ts.deletePolicy(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnAppMesh.Created = false
	return ts.cfg.EKSConfig.Sync()
}

const templatePolicy = `
---
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Amazon EKS AppMesh Controller policy'

Parameters:

  PolicyName:
    Type: String
    Description: The policy name for AppMesh Controller

  RoleNames:
    Type: CommaDelimitedList
    Description: The list of node instance roles

Resources:

  AppMeshControllerPolicy:
    Type: AWS::IAM::Policy
    Metadata:
      Comment: Minimal policy to allow worker node instance profile that allows the AppMesh Controller to make calls to AWS APIs on your behalf
    Properties:
      PolicyName: !Ref PolicyName
      PolicyDocument:
        Version: "2012-10-17"
        Statement:
          - Effect: Allow
            Action:
            - appmesh:*
            - servicediscovery:CreateService
            - servicediscovery:GetService
            - servicediscovery:RegisterInstance
            - servicediscovery:DeregisterInstance
            - servicediscovery:ListInstances
            - servicediscovery:ListNamespaces
            - servicediscovery:ListServices
            - route53:GetHealthCheck
            - route53:CreateHealthCheck
            - route53:UpdateHealthCheck
            - route53:ChangeResourceRecordSets
            - route53:DeleteHealthCheck
            Resource: "*"
      Roles: !Ref RoleNames

`

func (ts *tester) createPolicy() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID != "" {
		ts.cfg.Logger.Info("already created app mesh controller policy, ignoring")
		return nil
	}
	roleNames := make([]string, 0)
	if ts.cfg.EKSConfig.AddOnNodeGroups != nil && ts.cfg.EKSConfig.AddOnNodeGroups.RoleName != "" {
		roleNames = append(roleNames, ts.cfg.EKSConfig.AddOnNodeGroups.RoleName)
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups != nil && ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName != "" {
		roleNames = append(roleNames, ts.cfg.EKSConfig.AddOnManagedNodeGroups.RoleName)
	}
	if len(roleNames) == 0 {
		return errors.New("roles not found from node group or managed node group")
	}

	ts.cfg.Logger.Info("creating app mesh controller policy")
	stackName := ts.cfg.EKSConfig.Name + "-app-mesh-addOn"
	policyName := ts.cfg.EKSConfig.Name + "-app-mesh-policy"
	stackInput := &cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(templatePolicy),
		Tags: awscfn.NewTags(map[string]string{
			"Kind":                   "aws-k8s-tester",
			"Name":                   ts.cfg.EKSConfig.Name,
			"aws-k8s-tester-version": version.ReleaseVersion,
		}),
		Parameters: []*cloudformation.Parameter{
			{
				ParameterKey:   aws.String("PolicyName"),
				ParameterValue: aws.String(policyName),
			},
			{
				ParameterKey:   aws.String("RoleNames"),
				ParameterValue: aws.String(strings.Join(roleNames, ",")),
			},
		},
	}

	stackOutput, err := ts.cfg.CFNAPI.CreateStack(stackInput)
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Sig,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for app mesh controller policy creation (%v)", st.Error))
			ts.cfg.Logger.Warn("polling error", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}

	ts.cfg.Logger.Info("created app mesh controller policy",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID),
		zap.String("policy-name", policyName),
	)

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deletePolicy() error {
	if ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID == "" {
		ts.cfg.Logger.Info("empty app mesh controller policy, no need to delete")
		return nil
	}

	ts.cfg.Logger.Info("deleting app mesh controller policy",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID),
	)

	_, err := ts.cfg.CFNAPI.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := awscfn.Poll(
		ctx,
		make(chan struct{}),  // do not exit on stop
		make(chan os.Signal), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st awscfn.StackStatus
	for st = range ch {
		if st.Error != nil {
			cancel()
			ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("failed to wait for app mesh controller policy  deletion (%v)", st.Error))
			ts.cfg.Logger.Warn("polling error", zap.Error(st.Error))
		}
	}
	cancel()
	if st.Error != nil {
		return st.Error
	}
	ts.cfg.Logger.Info("app mesh controller policy",
		zap.String("cfn-stack-id", ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID),
	)
	ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID = ""

	return ts.cfg.EKSConfig.Sync()
}

const (
	chartRepoName = "eks"
	chartRepoURL  = "https://aws.github.io/eks-charts"

	chartNameController = "appmesh-controller"
	chartNameInjector   = "appmesh-inject"
)

// https://github.com/aws/eks-charts/blob/master/stable/appmesh-controller/values.yaml
func (ts *tester) createHelmController() error {
	// https://github.com/aws/eks-charts/blob/master/stable/appmesh-controller/values.yaml
	values := make(map[string]interface{})
	if ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage != "" {
		imageRepo, imageTag, err := splitImageRepoAndTag(ts.cfg.EKSConfig.AddOnAppMesh.ControllerImage)
		if err != nil {
			return err
		}
		values["image"] = map[string]interface{}{
			"repository": imageRepo,
			"tag":        imageTag,
		}
	}
	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Sig:            ts.cfg.Sig,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNameController,
		ReleaseName:    chartNameController,
		Values:         values,
		QueryFunc:      nil,
		QueryInterval:  30 * time.Second,
	})
}

func (ts *tester) deleteHelmController() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		ChartName:      chartNameController,
		ReleaseName:    chartNameController,
	})
}

// https://github.com/aws/eks-charts/blob/master/stable/appmesh-injector/values.yaml
func (ts *tester) createHelmInjector() error {
	values := make(map[string]interface{})
	if ts.cfg.EKSConfig.AddOnAppMesh.InjectorImage != "" {
		imageRepo, imageTag, err := splitImageRepoAndTag(ts.cfg.EKSConfig.AddOnAppMesh.InjectorImage)
		if err != nil {
			return err
		}
		values["image"] = map[string]interface{}{
			"repository": imageRepo,
			"tag":        imageTag,
		}
	}
	return helm.Install(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Sig:            ts.cfg.Sig,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartNameInjector,
		ReleaseName:    chartNameInjector,
		Values:         values,
		QueryFunc:      nil,
		QueryInterval:  30 * time.Second,
	})
}

func (ts *tester) deleteHelmInjector() error {
	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		ChartName:      chartNameInjector,
		ReleaseName:    chartNameInjector,
	})
}

// splitImageRepoAndTag parses a docker image in format <imageRepo>:<imageTag> into `imageRepo` and `imageTag`
func splitImageRepoAndTag(dockerImage string) (string, string, error) {
	parts := strings.Split(dockerImage, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("dockerImage expects <imageRepo>:<imageTag>, got: %s", dockerImage)
	}
	return parts[0], parts[1], nil
}
