package e2e

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
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
		batchJob := job.(*batchv1.Job)
		status := batchJob.Status
		spec := batchJob.Spec
		for _, condition := range status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == v1.ConditionTrue {
				return false, fmt.Errorf("job failed")
			}
		}
		if status.Succeeded != *spec.Completions {
			return false, nil
		}
		return true, nil
	}
}

func (c *ConditionExtension) AllNodesHaveNonZeroResourceCapacity(resourceLabel string) apimachinerywait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		nodeList := &v1.NodeList{}
		if err := c.resources.List(ctx, nodeList); err != nil {
			return false, fmt.Errorf("failed to list nodes: %w", err)
		}
		if len(nodeList.Items) == 0 {
			return false, fmt.Errorf("no nodes found in the cluster")
		}
		for _, node := range nodeList.Items {
			resource, ok := node.Status.Capacity[v1.ResourceName(resourceLabel)]
			if !ok {
				return false, nil
			}
			if resource.Value() <= 0 {
				return false, nil
			}
		}
		return true, nil
	}
}
