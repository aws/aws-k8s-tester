//go:build e2e

package disruptive

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/exec"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func getSleepPodTemplate(name string, targetNodeName string, duration string) corev1.Pod {
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
					Command: []string{"sleep", duration},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      targetNodeName,
			Resources: &corev1.ResourceRequirements{
				// set high pod limits to make sure the pod does not get
				// OOMKilled, and make requests equal to qualify the pod
				// for the Guaranteed Quality of Service class
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
	}
}

func TestGracefulReboot(t *testing.T) {
	terminationCanaryPodName := fmt.Sprintf("termination-canary-%d", time.Now().Unix())
	canaryPod := getSleepPodTemplate(terminationCanaryPodName, "", "infinity")
	bootIndicatorPodName := fmt.Sprintf("boot-detection-%d", time.Now().Unix())
	bootIndicatorPod := getSleepPodTemplate(bootIndicatorPodName, "", "infinity")

	feat := features.New("graceful-reboot").
		WithLabel("suite", "disruptive").
		Assess("Node gracefully reboots", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := cfg.Client().Resources().Create(ctx, &canaryPod); err != nil {
				t.Fatalf("Failed to create heartbeat pod: %v", err)
			}

			if err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).PodRunning(&canaryPod),
				wait.WithContext(ctx),
				wait.WithTimeout(5*time.Minute),
			); err != nil {
				t.Fatalf("Failed to wait for pod to go into running status %s: %v", terminationCanaryPodName, err)
			}

			var targetNode corev1.Node
			if err := cfg.Client().Resources().Get(ctx, canaryPod.Spec.NodeName, "", &targetNode); err != nil {
				t.Fatalf("failed to get node %s: %v", canaryPod.Spec.NodeName, err)
			}

			providerIDParts := strings.Split(targetNode.Spec.ProviderID, "/")
			instanceID := providerIDParts[len(providerIDParts)-1]
			t.Logf("Node %s corresponds to EC2 instance: %s", targetNode.Name, instanceID)

			ec2Client := ec2.NewFromConfig(awssdk.NewConfig())

			// TODO: make sure the exec starts before the reboot to promote better determinism
			t.Logf("Rebooting instance %s to test graceful reboot...", instanceID)
			_, err := ec2Client.RebootInstances(ctx, &ec2.RebootInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil {
				t.Fatalf("Failed to reboot EC2 instance %s: %v", instanceID, err)
			}
			t.Logf("Successfully initiated reboot of instance %s, waiting for pod %s to terminate...", instanceID, canaryPod.Name)

			t.Logf("Started exec into pod %s", terminationCanaryPodName)
			// Attempt to execute a blocking command in the pod until we get a 143, which would indicate a SIGTERM.
			// This a reliable way to check termination since it requires direct response from Kubelet
			var execOut, execErr bytes.Buffer
			err = cfg.Client().Resources().ExecInPod(ctx, "default", terminationCanaryPodName, terminationCanaryPodName, []string{"sleep", "infinity"}, &execOut, &execErr)
			if err != nil {
				if execErr, ok := err.(exec.CodeExitError); ok && execErr.Code == 143 {
					t.Logf("Pod %s was terminated", terminationCanaryPodName)
				} else {
					t.Fatalf("Got unexpected error terminating pod: %v", err)
				}
			}

			t.Logf("Waiting up to 10 minutes for node %s to become schedulable again", targetNode.Name)

			// Create a second pod, under the assumption that a new pod cannot be scheduled by a shutting down kubelet
			// that has already evicted other pods, so this one should only schedule with a new kubelet after boot
			bootIndicatorPod.Spec.NodeName = targetNode.Name
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
			} else {
				t.Logf("Successfully cleaned up pod %s", terminationCanaryPodName)
			}

			if err := cfg.Client().Resources().Delete(ctx, &bootIndicatorPod); err != nil {
				t.Logf("Failed to delete pod %s: %v", bootIndicatorPodName, err)
			} else {
				t.Logf("Successfully cleaned up pod %s", bootIndicatorPodName)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
