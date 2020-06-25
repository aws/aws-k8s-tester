package resource

import (
	"context"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"
	log "github.com/cihub/seelog"
	"github.com/davecgh/go-spew/spew"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type DeploymentManager struct {
	cs kubernetes.Interface
}

func NewDeploymentManager(cs kubernetes.Interface) *DeploymentManager {
	return &DeploymentManager{
		cs: cs,
	}
}

// WaitDeploymentReady waits for a deployment to be ready
func (m *DeploymentManager) WaitDeploymentReady(ctx context.Context, dp *appsv1.Deployment) (*appsv1.Deployment, error) {
	var (
		observedDP *appsv1.Deployment
		err        error
	)
	start := time.Now()

	return observedDP, wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		observedDP, err = m.cs.AppsV1().Deployments(dp.Namespace).Get(ctx, dp.Name, metav1.GetOptions{})
		cancel()
		if err != nil {
			return false, err
		}

		log.Debugf("%d / %d pods ready in namespace '%s' in deployment '%s' (%d seconds elapsed)",
			observedDP.Status.AvailableReplicas, observedDP.Status.Replicas, dp.Namespace,
			observedDP.ObjectMeta.Name, int(time.Since(start).Seconds()))

		if observedDP.Status.UpdatedReplicas == (*dp.Spec.Replicas) &&
			observedDP.Status.Replicas == (*dp.Spec.Replicas) &&
			observedDP.Status.AvailableReplicas == (*dp.Spec.Replicas) &&
			observedDP.Status.ObservedGeneration >= dp.Generation {
			return true, nil
		}
		return false, nil
	}, ctx.Done())
}

// WaitDeploymentDeleted waits for a deployment to be deleted
func (m *DeploymentManager) WaitDeploymentDeleted(ctx context.Context, dp *appsv1.Deployment) error {
	var (
		err error
	)
	return wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err = m.cs.AppsV1().Deployments(dp.Namespace).Get(ctx, dp.Name, metav1.GetOptions{})
		cancel()
		if err != nil {
			if serr, ok := err.(*kerrors.StatusError); ok {
				switch serr.ErrStatus.Reason {
				case "NotFound":
					return true, nil
				default:
					return false, err
				}
			}
			return false, err
		}
		return false, nil
	}, ctx.Done())
}

// ListDeploymentReplicaSets lists the replica sets in a deployment
func (m *DeploymentManager) ListDeploymentReplicaSets(dp *appsv1.Deployment) ([]*appsv1.ReplicaSet, error) {
	selector, err := metav1.LabelSelectorAsSelector(dp.Spec.Selector)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	replicaSetList, err := m.cs.AppsV1().ReplicaSets(dp.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	cancel()
	if err != nil {
		return nil, err
	}
	var controlled []*appsv1.ReplicaSet
	for _, rs := range replicaSetList.Items {
		if metav1.IsControlledBy(&rs, dp) {
			controlled = append(controlled, &rs)
		}
	}
	return controlled, nil
}

// ListReplicaSetPods lists the pods in the given replica sets
func (m *DeploymentManager) ListReplicaSetPods(replicaSets []*appsv1.ReplicaSet) ([]*corev1.Pod, error) {
	var pods []*corev1.Pod

	for _, rs := range replicaSets {
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		podList, err := m.cs.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
		cancel()
		if err != nil {
			return nil, err
		}
		for _, p := range podList.Items {
			if metav1.IsControlledBy(&p, rs) {
				pods = append(pods, &p)
			}
		}
	}
	return pods, nil
}

// DeploymentLogger logs replicas in the replicasets and pod statuses
func (m *DeploymentManager) DeploymentLogger(dp *appsv1.Deployment) error {
	replicaSets, err := m.ListDeploymentReplicaSets(dp)
	if err != nil {
		return err
	}
	for _, rs := range replicaSets {
		if rs.Status.AvailableReplicas == rs.Status.Replicas {
			log.Info(spew.Sprintf("ReplicaSet %q has %d/%d replicas", rs.Name, rs.Status.AvailableReplicas, rs.Status.Replicas))
		} else {
			log.Info(spew.Sprintf("ReplicaSet %q has %d/%d replicas %s:\n%+v", rs.Name, rs.Status.AvailableReplicas, rs.Status.Replicas, rs))
		}
	}
	pods, err := m.ListReplicaSetPods(replicaSets)
	if err != nil {
		return err
	}

	for _, p := range pods {
		if p.Status.Phase == "Running" {
			log.Info(spew.Sprintf("Pod %q is %s", p.Name, p.Status.Phase))
		} else {
			log.Info(spew.Sprintf("Pod %q is %s:\n%+v", p.Name, p.Status.Phase, p))
		}
	}
	return nil
}
