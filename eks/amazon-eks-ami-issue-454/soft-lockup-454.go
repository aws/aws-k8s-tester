/*
This is intended to test a soft lockup issue described here:
https://github.com/awslabs/amazon-eks-ami/issues/454
This is based off of the following repro:
https://github.com/mmerkes/eks-k8s-repro-assistant/tree/master/scenarios/decompression-loop
*/
package amazoneksamiissue454

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines test configuration
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

const (
	deploymentName           = "soft-lockup-454"
	decompressionLoopCommand = "yum install git -y; git clone https://github.com/aws/aws-sdk-go.git; tar cvf sdk.tar.gz aws-sdk-go; rm -rf aws-sdk-go && while true; do tar xvf sdk.tar.gz; sleep 5; done"
	nodeCheckWaitSeconds     = 120
	nodeCheckIntervalSeconds = 5
)

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnAmiSoftLockupIssue454() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	createStart := time.Now()

	ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Created = true
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace,
	); err != nil {
		return err
	}
	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.waitDeployment(); err != nil {
		return err
	}

	if err := ts.validateNodesStayHealthy(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnAmiSoftLockupIssue454() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete soft-lockup-issue-454 Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete soft-lockup-issue-454 namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating soft-lockup-454 Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": deploymentName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": deploymentName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": deploymentName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:    deploymentName,
									Image:   "centos:7",
									Command: []string{"bash"},
									Args:    []string{"-c", decompressionLoopCommand},
								},
							},
							NodeSelector: nodeSelector,
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create soft-luck-454 Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created soft-luck-454 Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting soft-luck-454 Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace).
		Delete(
			ctx,
			deploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete soft-luck-454 Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted soft-luck-454 Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() (err error) {
	timeout := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentReplicas)*time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = k8s_client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		20*time.Second,
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace,
		deploymentName,
		ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentReplicas,
		k8s_client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=" + ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.Namespace,
				"describe",
				"deployment",
				deploymentName,
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)
		}),
	)
	cancel()
	return err
}

func (ts *tester) validateNodesStayHealthy() (err error) {
	nodeSelector := ts.getNodeSelector()
	nodes, err := ts.cfg.K8SClient.ListNodes(
		1000,
		5*time.Second,
		k8s_client.WithFieldSelector(nodeSelector),
	)
	if err != nil {
		ts.cfg.Logger.Warn("list nodes failed", zap.Error(err))
		return err
	}

	start := time.Now()

	for {
		for _, node := range nodes {
			nodeName := node.GetName()
			ts.cfg.Logger.Info("checking node-info conditions", zap.String("node-name", nodeName))
			for _, cond := range node.Status.Conditions {
				if cond.Type != v1.NodeReady {
					continue
				}

				ts.cfg.Logger.Info("node info",
					zap.String("node-name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)

				if cond.Status != v1.ConditionTrue {
					return fmt.Errorf("node %s went unhealthy", nodeName)
				}
			}
		}

		if time.Since(start) >= nodeCheckWaitSeconds {
			ts.cfg.Logger.Info("All nodes stayed healthy")

			return nil
		}

		time.Sleep(nodeCheckIntervalSeconds * time.Second)
	}
}

func (ts *tester) getNodeSelector() string {
	if len(ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentNodeSelector) == 0 {
		return ""
	}

	nodeSelector := ts.cfg.EKSConfig.AddOnAmiSoftLockupIssue454.DeploymentNodeSelector
	b := new(bytes.Buffer)
	i := 0
	for key, value := range nodeSelector {
		if i != 0 {
			fmt.Fprintf(b, ",")
		}
		fmt.Fprintf(b, "%s=%s", key, value)
		i++
	}
	return b.String()
}
