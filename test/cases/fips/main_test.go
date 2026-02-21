//go:build e2e

package fips

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	pollInterval     = 5 * time.Second  // polling interval for waitForSeed and status checks
	seedTimeout      = 5 * time.Minute  // apk install + skopeo copy can be slow on first pull
	daemonSetTimeout = 2 * time.Minute  // per DaemonSet; image pulls vary by network
	logFetchTimeout  = 30 * time.Second // timeout for fetching pod logs
	// Worst-case Setup: 2x daemonSetTimeout (4m) + 2x seedTimeout (6m) = ~10m
)

var testenv env.Environment

func int64Ptr(i int64) *int64 { return &i }


func logDaemonSetDiagnostics(ctx context.Context, clientset *kubernetes.Clientset, dsName string) {
	log.Printf("=== Diagnostics for DaemonSet %s ===", dsName)
	pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
		LabelSelector: "name=" + dsName,
	})
	if err != nil {
		log.Printf("Failed to list pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		log.Printf("Pod %s: Phase=%s", pod.Name, pod.Status.Phase)
		for _, cond := range pod.Status.Conditions {
			log.Printf("  Condition %s: %s (Reason: %s)", cond.Type, cond.Status, cond.Reason)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			log.Printf("  Container %s: Ready=%v, RestartCount=%d", cs.Name, cs.Ready, cs.RestartCount)
			if cs.State.Waiting != nil {
				log.Printf("    Waiting: %s - %s", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
			if cs.State.Terminated != nil {
				log.Printf("    Terminated: %s - %s", cs.State.Terminated.Reason, cs.State.Terminated.Message)
			}
			if (cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff") || cs.RestartCount > 0 {
				req := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{
					Container: cs.Name,
					TailLines: int64Ptr(20),
				})
				stream, err := req.Stream(ctx)
				if err == nil {
					body, _ := io.ReadAll(stream)
					stream.Close()
					log.Printf("    Last logs:\n%s", string(body))
				}
			}
		}
	}
}


func logNodeInfo(ctx context.Context, clientset *kubernetes.Clientset) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Warning: could not list nodes: %v", err)
		return
	}
	for _, node := range nodes.Items {
		osImage := node.Status.NodeInfo.OSImage
		isFIPS := strings.Contains(strings.ToLower(osImage), "fips")
		log.Printf("Node %s: OS=%s, FIPS=%v", node.Name, osImage, isFIPS)
	}
}

// normally this will only take couple seconds.
func waitForSeed(ctx context.Context, clientset *kubernetes.Clientset, dsName string) error {
	log.Printf("Waiting for %s seed container to complete...", dsName)
	deadline := time.Now().Add(seedTimeout)
	var lastLogs string
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: "name=" + dsName,
		})
		if err != nil {
			return err
		}
		if len(pods.Items) == 0 {
			log.Printf("%s: no pods found yet, waiting...", dsName)
			time.Sleep(pollInterval)
			continue
		}
		allSeeded := true
		for _, pod := range pods.Items {
			req := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{
				Container: "seed-image",
			})
			logCtx, logCancel := context.WithTimeout(ctx, logFetchTimeout)
			stream, err := req.Stream(logCtx)
			if err != nil {
				logCancel()
				log.Printf("Failed to get logs for %s/%s: %v", dsName, pod.Name, err)
				allSeeded = false
				continue
			}
			body, _ := io.ReadAll(stream)
			stream.Close()
			logCancel()
			logs := string(body)
			if strings.Contains(logs, "level=fatal") {
				return fmt.Errorf("%s seed failed: %s", dsName, logs)
			}
			if !strings.Contains(logs, "Image seeded successfully") {
				allSeeded = false
				lastLogs = logs
			}
		}
		if allSeeded {
			log.Printf("%s seed completed successfully on all %d pods", dsName, len(pods.Items))
			return nil
		}
		log.Printf("%s seed still waiting... (got %d bytes of logs)", dsName, len(lastLogs))
		time.Sleep(pollInterval)
	}
	// Dump last logs on timeout
	if lastLogs != "" {
		log.Printf("%s seed timeout - last logs:\n%s", dsName, lastLogs)
	}
	return fmt.Errorf("%s seed did not complete within timeout", dsName)
}

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
			if err != nil {
				return ctx, fmt.Errorf("failed to create Kubernetes client: %w", err)
			}
			logNodeInfo(ctx, clientset)
			if err := fwext.ApplyManifests(config.Client().RESTConfig(), registryFIPSManifest); err != nil {
				return ctx, fmt.Errorf("failed to apply registry-fips manifest: %w", err)
			}
			log.Println("registry-fips DaemonSet deployed")

			if err := fwext.ApplyManifests(config.Client().RESTConfig(), registryNonFIPSManifest); err != nil {
				return ctx, fmt.Errorf("failed to apply registry-nonfips manifest: %w", err)
			}
			log.Println("registry-nonfips DaemonSet deployed")

			for _, name := range []string{"registry-fips", "registry-nonfips"} {
				ds := appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				}
				log.Printf("Waiting for %s DaemonSet to be ready...", name)
				err := wait.For(
					fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
					wait.WithContext(ctx),
					wait.WithTimeout(daemonSetTimeout),
				)
				if err != nil {
					logDaemonSetDiagnostics(ctx, clientset, name)
					return ctx, fmt.Errorf("%s DaemonSet not ready: %w", name, err)
				}
				log.Printf("%s DaemonSet is ready", name)
			}

			for _, dsName := range []string{"registry-fips", "registry-nonfips"} {
				if err := waitForSeed(ctx, clientset, dsName); err != nil {
					return ctx, fmt.Errorf("seed verification failed for %s: %w", dsName, err)
				}
			}

			if err := fwext.ApplyManifests(config.Client().RESTConfig(), testPodsManifest); err != nil {
				return ctx, fmt.Errorf("failed to apply test-pods manifest: %w", err)
			}
			log.Println("test pods deployed")

			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			fwext.DeleteManifests(config.Client().RESTConfig(), registryFIPSManifest)
			fwext.DeleteManifests(config.Client().RESTConfig(), registryNonFIPSManifest)
			fwext.DeleteManifests(config.Client().RESTConfig(), testPodsManifest)
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}
