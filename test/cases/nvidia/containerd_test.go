//go:build e2e

package nvidia

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	_ "embed"
)

//go:embed manifests/daemonset-containerd-check.yaml
var containerdCheckDS []byte

func TestContainerdConfig(t *testing.T) {
	feat := features.New("containerd-config-check").
		WithLabel("suite", "nvidia").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log.Println("[Setup] Applying containerd-check DaemonSet manifest.")
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), containerdCheckDS)
			if err != nil {
				t.Fatalf("failed to apply containerd-check DS: %v", err)
			}
			return ctx
		}).
		Assess("DaemonSet becomes ready", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			ds := &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "containerd-check",
					Namespace: "default",
				},
			}

			log.Println("[Assess] Waiting up to 1 minute for containerd-check DS to become Ready...")
			err := wait.For(
				fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(ds),
				wait.WithTimeout(1*time.Minute),
			)
			if err != nil {
				t.Logf("[Assess] containerd-check DS did not become Ready: %v", err)
				// Attempt to retrieve DS pod logs for debugging
				printDaemonSetPodLogs(ctx, cfg, "default", "app=containerd-check", t)
				t.Fatalf("containerd-check DS not Ready within 1 minute")
			}

			log.Println("[Assess] containerd-check DS is Ready.")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("[Teardown] Dumping DS logs before removal.")
			printDaemonSetPodLogs(ctx, cfg, "default", "app=containerd-check", t)

			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), containerdCheckDS)
			if err != nil {
				t.Fatalf("failed to delete containerd-check DS: %v", err)
			}
			t.Log("[Teardown] containerd-check DS removed successfully.")
			return ctx
		}).
		Feature()

	testenv.Test(t, feat)
}

func printDaemonSetPodLogs(ctx context.Context, cfg *envconf.Config, namespace, labelSelector string, t *testing.T) {
	clientset, err := getClientset(cfg.Client().RESTConfig())
	if err != nil {
		t.Logf("failed to create clientset: %v", err)
		return
	}

	// List pods by label
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		t.Logf("failed to list pods for label %q: %v", labelSelector, err)
		return
	}
	if len(pods.Items) == 0 {
		t.Log("No pods found for containerd-check DS.")
		return
	}

	// Fetch logs from each container in each pod
	for _, p := range pods.Items {
		t.Logf("Pod %s status: %s", p.Name, p.Status.Phase)
		for _, c := range p.Spec.Containers {
			logs, logErr := readPodLogs(ctx, clientset, p.Namespace, p.Name, c.Name)
			if logErr != nil {
				t.Logf("Failed to get logs from %s/%s: %v", p.Name, c.Name, logErr)
			} else {
				t.Logf("=== Logs from %s/%s ===\n%s", p.Name, c.Name, logs)
			}
		}
	}
}

func readPodLogs(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string) (string, error) {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	buf := make([]byte, 4096)
	var logs string
	for {
		n, rerr := stream.Read(buf)
		if n > 0 {
			logs += string(buf[:n])
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return logs, fmt.Errorf("error reading logs: %w", rerr)
		}
	}
	return logs, nil
}

func getClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return clientset, nil
}
