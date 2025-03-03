//go:build e2e

package quick

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/aws/aws-k8s-tester/internal/e2e"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func execInPod(config *rest.Config, namespace, podName, containerName string, command []string) (string, string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", err
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

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
							Image:   "ubuntu:noble-20241015",
							Command: []string{"sleep", "100000000000000"},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			}

			if err := cfg.Client().Resources().Create(ctx, pod); err != nil {
				t.Fatalf("[Assess] Failed to create pod: %v", err)
			}

			// Wait for pod to be running
			log.Printf("[Assess] Waiting for pod %s to be running...", podName)
			err := wait.For(
				e2e.NewConditionExtension(cfg.Client().Resources()).ResourceMatch(pod, func(object k8s.Object) bool {
					pod := object.(*corev1.Pod)
					return pod.Status.Phase == corev1.PodRunning
				}),
				wait.WithTimeout(2*time.Minute),
			)
			if err != nil {
				t.Fatalf("[Assess] Pod did not start running: %v", err)
			}

			// Execute commands in the pod
			execCmd := []string{
				"bash", "-c",
				`mkdir asd && 
				cd asd && 
				apt-get update && 
				apt-get install -y npm nodejs && 
				npm install karma@~6.4.0 --loglevel verbose`,
			}

			log.Printf("[Assess] Executing npm install in pod...")
			stdout, stderr, err := execInPod(cfg.Client().RESTConfig(), podNS, podName, "test-container", execCmd)
			if err != nil {
				t.Logf("stdout: %s", stdout)
				t.Logf("stderr: %s", stderr)
				t.Fatal("npm install failed or hung")
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
