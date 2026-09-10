//go:build e2e

package nvidia_fabric_manager

import (
	"context"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/test/common"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2ewait "sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestFabricManagerVersionMatchesDriver runs one pod per GPU node
// matching -nodeType, each of which cross-references the fabric-
// manager version parsed from /host/var/log/fabricmanager.log against
// the kernel driver version at /host/proc/driver/nvidia/version. The
// underlying workload is a Job (parallelism=completions=node count) so
// the test only passes when every pod exits 0 -- unlike the earlier
// DaemonSet variant which merely required pods to be Running.
func TestFabricManagerVersionMatchesDriver(t *testing.T) {
	feat := features.New("nvidia-fabric-manager-version-check").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			nodeCount, err := countGPUNodesOfType(ctx, cfg.Client().RESTConfig(), testConfig.NodeType)
			if err != nil {
				t.Fatalf("count GPU nodes of type %q: %v", testConfig.NodeType, err)
			}
			if nodeCount == 0 {
				t.Fatalf("no schedulable GPU nodes with instance-type=%q found; the Job would never complete", testConfig.NodeType)
			}
			rendered, err := fwext.RenderManifests(jobFabricManagerVersionCheckManifest, struct {
				NvidiaTestImage string
				NodeType        string
				NodeCount       int
			}{
				NvidiaTestImage: testConfig.NvidiaTestImage,
				NodeType:        testConfig.NodeType,
				NodeCount:       nodeCount,
			})
			if err != nil {
				t.Fatalf("render job: %v", err)
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Fatalf("apply job: %v", err)
			}
			ctx = context.WithValue(ctx, jobManifestKey{}, rendered)
			return ctx
		}).
		Assess("Job pods all succeed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "nvidia-fabric-manager-version-check", Namespace: "default"}}
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				e2ewait.WithTimeout(3*time.Minute),
			)
			if err != nil {
				t.Logf("Job did not complete successfully: %v", err)
				fwext.PrintDaemonSetPodLogs(t, ctx, cfg.Client().RESTConfig(), "default", "app=nvidia-fabric-manager-version-check")
				t.Fatalf("nvidia-fabric-manager-version-check Job did not have all pods Succeed within 3 minutes -- see pod logs for the 5.2 failure")
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, _ := ctx.Value(jobManifestKey{}).([]byte)
			if len(rendered) == 0 {
				return ctx
			}
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Errorf("delete job: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// TestFabricAndNVLinkInContainer covers the three in-container fabric/
// NVLink assertions in one pod that dispatches by node hardware:
// fabric state, NVLinks Active + expected count, no NVLinks on
// non-NVLink hardware. Each check either runs, prints SKIP for its
// class, or fails with a distinct exit code (50, 51, 52, 53 -- see
// the pod manifest).
func TestFabricAndNVLinkInContainer(t *testing.T) {
	feat := features.New("nvidia-fabric-nvlink-check").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Ask AWS how many GPUs this instance type has, so the pod
			// pins the whole node -- the fabric/NVLink assertions
			// need every GPU visible.
			info, err := common.GPUInfoForInstanceType(ctx, testConfig.NodeType, testConfig.Region)
			if err != nil {
				t.Fatalf("ec2:DescribeInstanceTypes(%q): %v", testConfig.NodeType, err)
			}
			rendered, err := fwext.RenderManifests(podFabricNVLinkCheckManifest, PodManifestTplVars{
				NvidiaTestImage: testConfig.NvidiaTestImage,
				GpuCount:        info.Count,
			})
			if err != nil {
				t.Fatalf("render manifest: %v", err)
			}
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Fatalf("apply pod: %v", err)
			}
			ctx = context.WithValue(ctx, renderedManifestKey{}, rendered)
			return ctx
		}).
		Assess("pod reaches Succeeded", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "nvidia-fabric-nvlink-check", Namespace: "default"}}
			err := e2ewait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).PodSucceeded(pod),
				e2ewait.WithTimeout(3*time.Minute),
			)
			if err != nil {
				if logs, lerr := fwext.ReadPodLogs(ctx, cfg.Client().RESTConfig(), "default", "nvidia-fabric-nvlink-check", "fabric-nvlink-check"); lerr == nil {
					t.Logf("--- pod nvidia-fabric-nvlink-check logs ---\n%s--- end pod logs ---", logs)
				} else {
					t.Logf("could not fetch pod logs for nvidia-fabric-nvlink-check: %v", lerr)
				}
				if err == wait.ErrWaitTimeout {
					t.Fatalf("fabric-nvlink pod did not complete within 3 minutes: %v", err)
				}
				t.Fatalf("fabric-nvlink pod ended in Failed phase: %v (exit 50=fabric state, 51=nvlink not Active, 52=nvlink count, 53=nvlinks on non-NVLink hw)", err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rendered, _ := ctx.Value(renderedManifestKey{}).([]byte)
			if len(rendered) == 0 {
				return ctx
			}
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), rendered); err != nil {
				t.Errorf("delete pod: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

// countGPUNodesOfType returns the number of Ready nodes carrying the
// requested EC2 instance-type label and advertising >=1 nvidia.com/gpu
// in .status.allocatable. It's used to size the fabric-manager Job's
// parallelism/completions so the Job pass condition is "every GPU node
// ran the check and exited 0".
func countGPUNodesOfType(ctx context.Context, restConfig *rest.Config, instanceType string) (int, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return 0, err
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node.kubernetes.io/instance-type=" + instanceType,
	})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, n := range nodes.Items {
		if !nodeIsReady(n) {
			continue
		}
		if q, ok := n.Status.Allocatable["nvidia.com/gpu"]; ok && q.Value() > 0 {
			count++
		}
	}
	return count, nil
}

func nodeIsReady(n v1.Node) bool {
	for _, c := range n.Status.Conditions {
		if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

type renderedManifestKey struct{}
type jobManifestKey struct{}
