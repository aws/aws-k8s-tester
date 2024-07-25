package frameworkext

import (
	"context"

	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
)

type ConditionExtension struct {
	resources *resources.Resources
}

func NewConditionExtension(r *resources.Resources) *ConditionExtension {
	return &ConditionExtension{resources: r}
}

// ResourceMatch is a helper function used to check if the resource under question has met a pre-defined state. This can
// be leveraged for checking fields on a resource that may not be immediately present upon creation.
func (c *ConditionExtension) ResourceMatch(obj k8s.Object, matchFetcher func(object k8s.Object) bool) apimachinerywait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		if err := c.resources.Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
			return false, err
		}
		return matchFetcher(obj), nil
	}
}

func (c *ConditionExtension) DaemonSetReady(daemonset k8s.Object) apimachinerywait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		if err := c.resources.Get(ctx, daemonset.GetName(), daemonset.GetNamespace(), daemonset); err != nil {
			return false, err
		}
		status := daemonset.(*appsv1.DaemonSet).Status
		if status.NumberReady == status.DesiredNumberScheduled && status.NumberUnavailable == 0 {
			done = true
		}
		return
	}
}

func (c *ConditionExtension) JobSucceeded(job k8s.Object) apimachinerywait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		if err := c.resources.Get(ctx, job.GetName(), job.GetNamespace(), job); err != nil {
			return false, err
		}
		status := job.(*batchv1.Job).Status
		spec := job.(*batchv1.Job).Spec
		if status.Succeeded != *spec.Completions {
			return false, nil
		}
		return true, nil
	}
}

func (c *ConditionExtension) MpiJobSucceeded(obj k8s.Object) apimachinerywait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		if err := c.resources.Get(ctx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
			return false, err
		}
		j := obj.(*kubeflowv2beta1.MPIJob)
		for _, c := range j.Status.Conditions {
			if c.Type == kubeflowv2beta1.JobSucceeded {
				return true, nil
			}
		}
		return false, nil
	}
}
