// Package phpapache implements PHP Apache
// with a simple PHP app.
package phpapache

import (
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

// Config defines configuration.
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
	phpApacheDeploymentName = "php-apache-deployment"
	phpApacheAppName        = "php-apache"
	phpApacheAppImageName   = "pjlewis/php-apache"
)

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnPHPApache() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnPHPApache.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", pkgName))
	ts.cfg.EKSConfig.AddOnPHPApache.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnPHPApache.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnPHPApache.Namespace,
	); err != nil {
		return err
	}
	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.waitDeployment(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnPHPApache() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnPHPApache.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", pkgName))
	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnPHPApache.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete php-apache Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnPHPApache.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnPHPApache.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	var nodeSelector map[string]string
	if len(ts.cfg.EKSConfig.AddOnPHPApache.DeploymentNodeSelector) > 0 {
		nodeSelector = ts.cfg.EKSConfig.AddOnPHPApache.DeploymentNodeSelector
	} else {
		nodeSelector = nil
	}
	ts.cfg.Logger.Info("creating php-apache Deployment", zap.Any("node-selector", nodeSelector))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnPHPApache.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      phpApacheDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnPHPApache.Namespace,
					Labels: map[string]string{
						"app.kubernetes.io/name": phpApacheAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnPHPApache.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": phpApacheAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": phpApacheAppName,
							},
						},
						Spec: v1.PodSpec{
							RestartPolicy: v1.RestartPolicyAlways,
							Containers: []v1.Container{
								{
									Name:            phpApacheAppName,
									Image:           phpApacheAppImageName,
									ImagePullPolicy: v1.PullAlways,
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
		return fmt.Errorf("failed to create php-apache Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("created php-apache Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting php-apache Deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnPHPApache.Namespace).
		Delete(
			ctx,
			phpApacheDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete php-apache Deployment (%v)", err)
	}

	ts.cfg.Logger.Info("deleted php-apache Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for php-apache Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnPHPApache.Namespace,
		"describe",
		"deployment",
		phpApacheDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 7*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnPHPApache.DeploymentReplicas)*time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnPHPApache.Namespace).
			Get(ctx, phpApacheDeploymentName, metav1.GetOptions{})
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

		// TODO: remove the pod with "Error: ImagePullBackOff"
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
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		// TODO: remove this hack and handle "Error: ImagePullBackOff"
		if available && dresp.Status.AvailableReplicas+1 >= ts.cfg.EKSConfig.AddOnPHPApache.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("deployment not ready")
	}

	ts.cfg.Logger.Info("waited for php-apache Deployment")
	return ts.cfg.EKSConfig.Sync()
}
