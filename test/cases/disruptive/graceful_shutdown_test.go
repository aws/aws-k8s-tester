//go:build e2e

package disruptive

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	"github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// getPodLogs retrieves logs from a pod using kubernetes clientset
func getPodLogs(ctx context.Context, cfg *envconf.Config, podName, namespace string) (string, error) {
	client, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		return "", err
	}

	req := client.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	logs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	var result strings.Builder
	_, err = io.Copy(&result, logs)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

// checkLogPattern checks if a log pattern exists in the pod logs
func checkLogPattern(ctx context.Context, cfg *envconf.Config, podName, namespace, pattern string) (bool, error) {
	logs, err := getPodLogs(ctx, cfg, podName, namespace)
	if err != nil {
		return false, err
	}

	matched, err := regexp.MatchString(pattern, logs)
	if err != nil {
		return false, err
	}

	return matched, nil
}

// countLogMatches counts how many times a pattern appears in the logs
func countLogMatches(ctx context.Context, cfg *envconf.Config, podName, namespace, pattern string) (int, error) {
	logs, err := getPodLogs(ctx, cfg, podName, namespace)
	if err != nil {
		return 0, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, err
	}

	matches := re.FindAllString(logs, -1)
	return len(matches), nil
}

func TestKubeletGracefulShutdown(t *testing.T) {
	feat := features.New("kubelet-graceful-shutdown").
		WithLabel("suite", "disruptive").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Setting up Kubelet Graceful Shutdown test...")
			return ctx
		}).
		Assess("Kubelet gracefully shuts down pods during node termination", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Create heartbeat pod that will log its status
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("graceful-shutdown-test-%d", time.Now().Unix()),
					Namespace: "default",
					Labels: map[string]string{
						"app": "graceful-shutdown-test",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "heartbeat-container",
							Image:   "public.ecr.aws/amazonlinux/amazonlinux:2023",
							Command: []string{"/usr/bin/bash", "-c"},
							Args: []string{`
								set -x
								echo "[GRACEFUL-TEST] Starting graceful shutdown test pod..."
								
								function handle_sigterm() {
									echo "[GRACEFUL-TEST] $(date): SIGTERM-RECEIVED - starting graceful shutdown period"
									# Continue heartbeating until we are SIGKILL-d
									start_time=$(date +%s)
									while true; do
										current_time=$(date +%s)
										elapsed=$((current_time - start_time))
										echo "[GRACEFUL-TEST] $(date): HEARTBEAT-AFTER-SIGTERM elapsed=${elapsed}s"
										sleep 1
									done
								}
								
								trap handle_sigterm TERM
								
								# Initial heartbeat to show pod is running
								echo "[GRACEFUL-TEST] $(date): POD-STARTED - pod started successfully"
								
								# Keep running and heartbeating until terminated
								counter=0
								while true; do
									echo "[GRACEFUL-TEST] $(date): NORMAL-HEARTBEAT counter=$counter"
									counter=$((counter + 1))
									sleep 10
								done
							`},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: pointer.Int64(150), // 2.5 minutes to allow for graceful shutdown testing
				},
			}

			if err := cfg.Client().Resources().Create(ctx, pod); err != nil {
				t.Fatalf("[Assess] Failed to create heartbeat pod: %v", err)
			}
			log.Printf("[Assess] Created heartbeat pod: %s", pod.Name)

			// Store pod name in context for cleanup
			ctx = context.WithValue(ctx, "podName", pod.Name)

			log.Printf("[Assess] Waiting for pod %s to start running...", pod.Name)
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

			// Wait a bit for initial heartbeats
			log.Printf("[Assess] Waiting for initial heartbeats...")
			time.Sleep(30 * time.Second)

			// Verify pod started successfully by checking logs
			podStarted, err := checkLogPattern(ctx, cfg, pod.Name, pod.Namespace, `POD-STARTED`)
			if err != nil {
				t.Fatalf("[Assess] Failed to check pod logs: %v", err)
			}
			if !podStarted {
				t.Fatalf("[Assess] Pod did not log successful startup")
			}
			log.Printf("[Assess] ✓ Pod startup confirmed via logs")

			// Get the node the pod is running on
			if err := cfg.Client().Resources().Get(ctx, pod.Name, pod.Namespace, pod); err != nil {
				t.Fatalf("[Assess] Failed to get pod details: %v", err)
			}

			nodeName := pod.Spec.NodeName
			if nodeName == "" {
				t.Fatalf("[Assess] Pod is not scheduled to any node")
			}
			log.Printf("[Assess] Pod is running on node: %s", nodeName)

			// Get the EC2 instance ID for this node
			var node corev1.Node
			if err := cfg.Client().Resources().Get(ctx, nodeName, "", &node); err != nil {
				t.Fatalf("[Assess] Failed to get node %s: %v", nodeName, err)
			}
			providerID := node.Spec.ProviderID
			if providerID == "" {
				t.Fatalf("[Assess] Node %s has no providerID", nodeName)
			}
			parts := strings.Split(providerID, "/")
			if len(parts) < 2 {
				t.Fatalf("[Assess] Invalid providerID format: %s", providerID)
			}
			instanceID := parts[len(parts)-1]
			log.Printf("[Assess] Node %s corresponds to EC2 instance: %s", nodeName, instanceID)

			// Terminate the EC2 instance
			log.Printf("[Assess] Terminating EC2 instance %s to test graceful shutdown...", instanceID)
			ec2Client := ec2.NewFromConfig(awssdk.NewConfig())
			_, err = ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil {
				t.Fatalf("[Assess] Failed to terminate EC2 instance %s: %v", instanceID, err)
			}
			log.Printf("[Assess] Successfully initiated termination of instance %s", instanceID)

			// Wait and monitor the graceful shutdown process via logs
			log.Printf("[Assess] Monitoring graceful shutdown process for 3 minutes...")

			// Wait for SIGTERM to be received (should happen within 60 seconds)
			sigtermReceived := false
			for i := 0; i < 30; i++ {
				received, err := checkLogPattern(ctx, cfg, pod.Name, pod.Namespace, `SIGTERM-RECEIVED`)
				if err != nil {
					log.Printf("[Assess] Warning: Failed to check logs: %v", err)
				} else if received {
					sigtermReceived = true
					log.Printf("[Assess] ✓ SIGTERM received by pod (detected after %d seconds)", i*2)
					break
				}
				time.Sleep(2 * time.Second)
			}

			if !sigtermReceived {
				t.Fatalf("[Assess] Pod did not receive SIGTERM within 60 seconds of instance termination")
			}

			// Monitor heartbeats for the next 2+ minutes to verify graceful shutdown behavior
			log.Printf("[Assess] Verifying pod continues running during graceful shutdown period...")
			gracefulShutdownStart := time.Now()

			var heartbeatsAfterSigterm int
			for time.Since(gracefulShutdownStart) < 2*time.Minute { // Monitor for 2 minutes
				// Count heartbeats after SIGTERM
				matches, err := countLogMatches(ctx, cfg, pod.Name, pod.Namespace, `HEARTBEAT-AFTER-SIGTERM`)
				if err != nil {
					log.Printf("[Assess] Warning: Failed to count heartbeats: %v", err)
				} else if matches > 0 {
					log.Printf("[Assess] ✓ Pod still running after SIGTERM (%d heartbeats logged)", matches)
					heartbeatsAfterSigterm = matches
				}

				time.Sleep(1 * time.Second)
			}

			// Verify we got heartbeats during the graceful shutdown period
			// These happen once a second, so we should observe at least 110 of them for a 2 minute grace period
			if heartbeatsAfterSigterm < 110 {
				t.Fatalf("[Assess] Expected at least 110 heartbeats during graceful shutdown, got %d", heartbeatsAfterSigterm)
			}

			log.Printf("[Assess] ✓ Pod continued running and heartbeating for graceful shutdown period")
			log.Printf("[Assess] ✓ Total heartbeats after SIGTERM: %d", heartbeatsAfterSigterm)

			// Check for graceful exit
			gracefulExit, err := checkLogPattern(ctx, cfg, pod.Name, pod.Namespace, `GRACEFUL-EXIT`)
			if err != nil {
				log.Printf("[Assess] Warning: Failed to check for graceful exit: %v", err)
			} else if gracefulExit {
				log.Printf("[Assess] ✓ Pod logged graceful exit")
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			podName, ok := ctx.Value("podName").(string)
			if !ok {
				log.Printf("[Teardown] No pod name in context, nothing to clean up")
				return ctx
			}

			log.Printf("[Teardown] Cleaning up test pod %s...", podName)

			// Get final logs for debugging if needed
			logs, err := getPodLogs(ctx, cfg, podName, "default")
			if err != nil {
				log.Printf("[Teardown] Warning: Failed to get final logs: %v", err)
			} else {
				log.Printf("[Teardown] Final pod logs:\n%s", logs)
			}

			// Delete the pod (it may already be terminated)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: "default",
				},
			}
			if err := cfg.Client().Resources().Delete(ctx, pod); err != nil {
				log.Printf("[Teardown] Warning: Failed to delete pod %s: %v", podName, err)
			} else {
				log.Printf("[Teardown] Successfully cleaned up pod %s", podName)
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}
