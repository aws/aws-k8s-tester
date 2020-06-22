// Package appmesh implements App Mesh add-on.
package appmesh

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/helm"
	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/cfn"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines AppMesh configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	CFNAPI    cloudformationiface.CloudFormationAPI
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnAppMesh() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnAppMesh.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnAppMesh.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createPolicy(); err != nil {
		return err
	}
	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
	); err != nil {
		return err
	}
	if err := helm.RepoAdd(ts.cfg.Logger, chartRepoName, chartRepoURL); err != nil {
		return err
	}
	if err := ts.createController(); err != nil {
		return err
	}
	if err := ts.createInjector(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnAppMesh() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnAppMesh.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteInjector(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ts.deleteController(); err != nil {
		errs = append(errs, err.Error())
	}

	time.Sleep(10 * time.Second)

	getAllArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		"get",
		"all",
	}
	getAllCmd := strings.Join(getAllArgs, " ")

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithQueryFunc(func() {
			println()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, getAllArgs[0], getAllArgs[1:]...).CombinedOutput()
			cancel()
			out := strings.TrimSpace(string(output))
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl get all' failed", zap.Error(err))
			} else {
				fmt.Printf("\n\n'%s' output:\n\n%s\n\n", getAllCmd, out)
			}
		}),
		k8s_client.WithForceDelete(true),
	); err != nil {
		// TODO
		// errs = append(errs, fmt.Sprintf("failed to delete AppMesh namespace (%v)", err))
		ts.cfg.Logger.Warn("failed to delete AppMesh namespace", zap.Error(err))
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

	if err := ioutil.WriteFile(ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackYAMLFilePath, []byte(templatePolicy), 0400); err != nil {
		return err
	}
	policyName := ts.cfg.EKSConfig.Name + "-appmesh-policy"
	ts.cfg.Logger.Info("creating app mesh controller policy",
		zap.String("policy-name", policyName),
		zap.String("policy-cfn-file-path", ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackYAMLFilePath),
	)
	stackOutput, err := ts.cfg.CFNAPI.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(policyName),
		Capabilities: aws.StringSlice([]string{"CAPABILITY_NAMED_IAM"}),
		OnFailure:    aws.String(cloudformation.OnFailureDelete),
		TemplateBody: aws.String(templatePolicy),
		Tags: cfn.NewTags(map[string]string{
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
	})
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID = aws.StringValue(stackOutput.StackId)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	ch := cfn.Poll(
		ctx,
		ts.cfg.Stopc,
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID,
		cloudformation.ResourceStatusCreateComplete,
		25*time.Second,
		10*time.Second,
	)
	var st cfn.StackStatus
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
	ch := cfn.Poll(
		ctx,
		make(chan struct{}), // do not exit on stop
		ts.cfg.Logger,
		ts.cfg.CFNAPI,
		ts.cfg.EKSConfig.AddOnAppMesh.PolicyCFNStackID,
		cloudformation.ResourceStatusDeleteComplete,
		25*time.Second,
		10*time.Second,
	)
	var st cfn.StackStatus
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
func (ts *tester) createController() error {
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

func (ts *tester) deleteController() (err error) {
	foreground := metav1.DeletePropagationForeground

	ts.cfg.Logger.Info("deleting AppMesh controller Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
		Delete(
			ctx,
			chartNameController,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("deleted AppMesh controller deployment")
	}
	time.Sleep(20 * time.Second)

	ts.cfg.Logger.Info("deleting all ReplicaSets")
	var rs *v1.ReplicaSetList
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	rs, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		ReplicaSets(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
		List(ctx, metav1.ListOptions{})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to list replicasets", zap.Error(err))
	} else {
		for _, v := range rs.Items {
			name := v.Name
			ts.cfg.Logger.Info("deleting replicaset", zap.String("name", name))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			err = ts.cfg.K8SClient.KubernetesClientSet().
				AppsV1().
				ReplicaSets(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
				Delete(
					ctx,
					name,
					metav1.DeleteOptions{
						GracePeriodSeconds: aws.Int64(0),
						PropagationPolicy:  &foreground,
					},
				)
			cancel()
			if err != nil && !api_errors.IsNotFound(err) {
				ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
			} else {
				ts.cfg.Logger.Info("deleted AppMesh injector replicaset", zap.String("name", name))
			}
		}
	}
	time.Sleep(20 * time.Second)

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
func (ts *tester) createInjector() error {
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

/*
$ /tmp/kubectl-test-v1.17.6 --kubeconfig=/tmp/proudpcgaspvcpn.kubeconfig.yaml -n eks-2020061416-prime6774tws-appmesh get all

NAME                                      READY   STATUS    RESTARTS   AGE
pod/appmesh-controller-55c7bdf448-s79zr   1/1     Running   0          2m16s
pod/appmesh-inject-6fb67dbb44-jfqvq       1/1     Running   0          2m

NAME                     TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)   AGE
service/appmesh-inject   ClusterIP   10.100.67.220   <none>        443/TCP   2m

NAME                                 READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/appmesh-controller   1/1     1            1           2m16s
deployment.apps/appmesh-inject       1/1     1            1           2m

NAME                                            DESIRED   CURRENT   READY   AGE
replicaset.apps/appmesh-controller-55c7bdf448   1         1         1       2m16s
replicaset.apps/appmesh-inject-6fb67dbb44       1         1         1       2m
*/

func (ts *tester) deleteInjector() (err error) {
	foreground := metav1.DeletePropagationForeground

	ts.cfg.Logger.Info("deleting AppMesh injector Service")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
		Delete(
			ctx,
			chartNameInjector,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("deleted AppMesh injector Service")
	}
	time.Sleep(20 * time.Second)

	ts.cfg.Logger.Info("deleting AppMesh injector Deployment")
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
		Delete(
			ctx,
			chartNameInjector,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !api_errors.IsNotFound(err) {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
	} else {
		ts.cfg.Logger.Info("deleted AppMesh injector deployment")
	}
	time.Sleep(20 * time.Second)

	ts.cfg.Logger.Info("deleting all ReplicaSets")
	var rs *v1.ReplicaSetList
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	rs, err = ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		ReplicaSets(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
		List(ctx, metav1.ListOptions{})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("failed to list replicasets", zap.Error(err))
	} else {
		for _, v := range rs.Items {
			name := v.Name
			ts.cfg.Logger.Info("deleting replicaset", zap.String("name", name))
			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			err = ts.cfg.K8SClient.KubernetesClientSet().
				AppsV1().
				ReplicaSets(ts.cfg.EKSConfig.AddOnAppMesh.Namespace).
				Delete(
					ctx,
					name,
					metav1.DeleteOptions{
						GracePeriodSeconds: aws.Int64(0),
						PropagationPolicy:  &foreground,
					},
				)
			cancel()
			if err != nil && !api_errors.IsNotFound(err) {
				ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
			} else {
				ts.cfg.Logger.Info("deleted AppMesh injector replicaset", zap.String("name", name))
			}
		}
	}
	time.Sleep(20 * time.Second)

	return helm.Uninstall(helm.InstallConfig{
		Logger:         ts.cfg.Logger,
		Timeout:        15 * time.Minute,
		KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		Namespace:      ts.cfg.EKSConfig.AddOnAppMesh.Namespace,
		ChartName:      chartNameInjector,
		ReleaseName:    chartNameInjector,
	})
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnAppMesh() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnAppMesh.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", pkgName))
	return nil
}

// splitImageRepoAndTag parses a docker image in format <imageRepo>:<imageTag> into `imageRepo` and `imageTag`
func splitImageRepoAndTag(dockerImage string) (string, string, error) {
	parts := strings.Split(dockerImage, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("dockerImage expects <imageRepo>:<imageTag>, got: %s", dockerImage)
	}
	return parts[0], parts[1], nil
}
