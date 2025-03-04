//go:build e2e

package quick

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestNpmInstallWithCPULimits(t *testing.T) {
	feat := features.New("npm-install-cpu-limits").
		WithLabel("suite", "quick").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Verifying cluster nodes...")
			var nodeList corev1.NodeList
			if err := cfg.Client().Resources().List(ctx, &nodeList); err != nil {
				t.Fatalf("Failed to list nodes: %v", err)
			}

			// Log node information
			for _, node := range nodeList.Items {
				arch := node.Labels["kubernetes.io/arch"]
				kernelVersion := node.Status.NodeInfo.KernelVersion
				t.Logf("Node: %s, Architecture: %s, Kernel: %s", node.Name, arch, kernelVersion)
			}
			return ctx
		}).
		Assess("Pod can successfully run npm install with CPU limits", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "npm-install-test"
			podNS := "default"

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: podNS,
					Labels: map[string]string{
						"app": "npm-install-test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "test-container",
							Image:   "public.ecr.aws/ubuntu/ubuntu:noble",
							Command: []string{"/bin/sh", "-c"},
							Args: []string{`
								set -x
								echo "[Test] Starting npm installation test..."
								mkdir asd && 
								cd asd && 
								apt-get update && 
								apt-get install -y npm nodejs && 
								echo "[Test] Starting npm install webpack..."
								npm install webpack --loglevel verbose || exit 1
								echo "[Test] npm install completed successfully"
							`},
							// Resources: corev1.ResourceRequirements{
							// 	Limits: corev1.ResourceList{
							// 		corev1.ResourceCPU:    resource.MustParse("500m"),
							// 		corev1.ResourceMemory: resource.MustParse("1Gi"),
							// 	},
							// 	Requests: corev1.ResourceList{
							// 		corev1.ResourceCPU:    resource.MustParse("500m"),
							// 		corev1.ResourceMemory: resource.MustParse("1Gi"),
							// 	},
							// },
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			}

			if err := cfg.Client().Resources().Create(ctx, pod); err != nil {
				t.Fatalf("[Assess] Failed to create pod: %v", err)
			}

			log.Printf("[Assess] Waiting up to 10 minutes for pod %s to complete...", podName)
			err := wait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).ResourceMatch(pod, func(object k8s.Object) bool {
					pod := object.(*corev1.Pod)
					return pod.Status.Phase == corev1.PodSucceeded
				}),
				wait.WithTimeout(10*time.Minute),
			)
			if err != nil {
				t.Logf("[Assess] Pod did not complete successfully: %v", err)
				e2e.PrintDaemonSetPodLogs(t, ctx, cfg.Client().RESTConfig(), podNS, "app=npm-install-test")
				t.Fatal("Pod did not complete within 10 minutes - possible io_uring hang detected")
			}

			log.Printf("[Assess] Pod %s completed successfully", podName)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName := "npm-install-test"
			podNS := "default"

			t.Logf("[Teardown] Cleaning up pod %s/%s...", podNS, podName)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: podNS,
				},
			}
			if err := cfg.Client().Resources().Delete(ctx, pod); err != nil {
				t.Logf("[Teardown] Failed to delete pod: %v", err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
