//go:build e2e

package disruptive

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	fwext "github.com/aws/aws-k8s-tester/internal/e2e"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func getSleepPodTemplate(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    name,
					Image:   "public.ecr.aws/amazonlinux/amazonlinux:2023",
					Command: []string{"sleep", "infinity"},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func TestGracefulReboot(t *testing.T) {
	terminationCanaryPodName := fmt.Sprintf("termination-canary-%d", time.Now().Unix())
	canaryPod := getSleepPodTemplate(terminationCanaryPodName)
	bootIndicatorPodName := fmt.Sprintf("boot-detection-%d", time.Now().Unix())
	bootIndicatorPod := getSleepPodTemplate(bootIndicatorPodName)

	feat := features.New("graceful-reboot").
		WithLabel("suite", "disruptive").
		Assess("Node gracefully reboots", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create an initial pod to allow the default scheduler to do the work of identifying a healthy node.
			// Starting with a healthy node is essential to the test, as the only expectation is for the node to
			// return to its same initial state after the reboot.
			if err := cfg.Client().Resources().Create(ctx, &canaryPod); err != nil {
				t.Fatalf("Failed to create heartbeat pod: %v", err)
			}

			if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).PodRunning(&canaryPod),
				wait.WithContext(ctx),
				wait.WithTimeout(5*time.Minute),
			); err != nil {
				t.Fatalf("Failed to wait for pod %s to go into running status: %v", terminationCanaryPodName, err)
			}

			var targetNode corev1.Node
			if err := cfg.Client().Resources().Get(ctx, canaryPod.Spec.NodeName, "", &targetNode); err != nil {
				t.Fatalf("Failed to get node %s: %v", canaryPod.Spec.NodeName, err)
			}

			t.Logf("Pod %s is running on node %s", terminationCanaryPodName, targetNode.Name)

			// Do an initial check of the /healthz endpoint reachability to ensure we can rely on it later.
			// This might fail even if the node is healthy if, for example, the node's security group rules
			// do not allow ingress traffic from the control plane
			kubeletResponsive, err := fwext.KubeletIsResponsive(ctx, cfg.Client().RESTConfig(), targetNode.Name)
			if err != nil || !kubeletResponsive {
				t.Fatalf("Node %s is not responding to initial /healthz checks: %v", targetNode.Name, err)
			}

			providerIDParts := strings.Split(targetNode.Spec.ProviderID, "/")
			instanceID := providerIDParts[len(providerIDParts)-1]
			t.Logf("Rebooting underlying instance %s for node %s...", instanceID, targetNode.Name)

			ec2Client := ec2.NewFromConfig(awssdk.NewConfig())
			if _, err := ec2Client.RebootInstances(ctx, &ec2.RebootInstancesInput{
				InstanceIds: []string{instanceID},
			}); err != nil {
				t.Fatalf("Failed to reboot instance %s: %v", instanceID, err)
			}

			t.Logf("Successfully triggered reboot of instance %s, waiting for kubelet to become unresponsive...", instanceID)

			kubeletShutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			// Use kubelet health probes as the signal for instance shutdown. Since the health endpoint
			// could previously be reached, a refused connection implies kubelet was killed.
			for kubeletResponsive {
				select {
				case <-kubeletShutdownCtx.Done():
					t.Fatalf("Failed to wait for kubelet to become unresponsive: %v", ctx.Err())
				case <-time.Tick(1 * time.Second):
					if kubeletResponsive, err = fwext.KubeletIsResponsive(ctx, cfg.Client().RESTConfig(), targetNode.Name); err != nil {
						t.Fatalf("Unpexected error while monitoring kubelet on node %s: %v", targetNode.Name, err)
					}
				}
			}

			t.Logf("Node %s has become unresponsive, waiting for the node to become schedulable again...", targetNode.Name)

			// Create a second pod, we will rely on this pod starting to run as an indication of a healthy state.
			// Since kubelet was killed at this point, we know the reboot must complete and kubelet must start
			// again for this pod to start running.
			bootIndicatorPod.Spec.NodeSelector = map[string]string{
				"kubernetes.io/hostname": targetNode.Name,
			}
			if err := cfg.Client().Resources().Create(ctx, &bootIndicatorPod); err != nil {
				t.Fatalf("Failed to create boot indicator pod: %v", err)
			}

			if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).PodRunning(&bootIndicatorPod),
				wait.WithContext(ctx),
				wait.WithTimeout(10*time.Minute), // TODO: bring down this value after collecting some more data
			); err != nil {
				t.Fatalf("Failed to wait for pod to go into running status %s: %v", bootIndicatorPodName, err)
			}

			t.Logf("Node %s became ready and schedulable within %v!", targetNode.Name, time.Since(bootIndicatorPod.CreationTimestamp.Time))
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := cfg.Client().Resources().Delete(ctx, &canaryPod); err != nil {
				t.Logf("Failed to delete pod %s: %v", terminationCanaryPodName, err)
			}

			if err := cfg.Client().Resources().Delete(ctx, &bootIndicatorPod); err != nil {
				t.Logf("Failed to delete pod %s: %v", bootIndicatorPodName, err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
