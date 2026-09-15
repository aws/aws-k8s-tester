//go:build e2e

package nvidia_dra

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const gpuPresentLabelKey = "nvidia.com/gpu.present"

// TestGPUPresentLabel asserts that GPU nodes carry nvidia.com/gpu.present=true without
// this suite having applied it. The NVIDIA DRA kubelet plugin gates its DaemonSet on
// that label through nodeAffinity, so a node image that does not set it leaves the
// plugin scheduled onto zero nodes and GPUs unallocatable.
func TestGPUPresentLabel(t *testing.T) {
	if !*autoModeEnabled {
		t.Skip("-autoModeEnabled not set; the suite labels the nodes itself, so this assertion would be vacuous")
	}

	feat := features.New("gpu-present-label").
		WithLabel("suite", "nvidia-dra").
		Assess("GPU nodes are labeled nvidia.com/gpu.present=true by the node image", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: "node.kubernetes.io/instance-type=" + *nodeType,
			})
			if err != nil {
				t.Fatalf("failed to list %s nodes: %v", *nodeType, err)
			}
			if len(nodes.Items) == 0 {
				t.Fatalf("no nodes of type %s found", *nodeType)
			}
			for _, node := range nodes.Items {
				if got := node.Labels[gpuPresentLabelKey]; got != "true" {
					t.Errorf("node %s: %s = %q, want %q", node.Name, gpuPresentLabelKey, got, "true")
				}
			}
			t.Logf("all %d %s node(s) carry %s=true", len(nodes.Items), *nodeType, gpuPresentLabelKey)
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
