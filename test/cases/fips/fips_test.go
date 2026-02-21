//go:build e2e

package fips

import (
	"context"
	_ "embed"
	"io"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	pullTimeout   = 5 * time.Minute
	rejectTimeout = 2 * time.Minute
)

var (
	//go:embed manifests/registry-fips.yaml
	registryFIPSManifest []byte
	//go:embed manifests/registry-nonfips.yaml
	registryNonFIPSManifest []byte

	//go:embed manifests/test-pods.yaml
	testPodsManifest []byte
)

func verifyNonfipsCipherRejection(ctx context.Context, t *testing.T, cfg *envconf.Config) {
	t.Helper()
	clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatalf("could not create clientset for log verification: %v", err)
	}
	logCtx, logCancel := context.WithTimeout(ctx, logFetchTimeout)
	defer logCancel()
	pods, err := clientset.CoreV1().Pods("default").List(logCtx, metav1.ListOptions{
		LabelSelector: "name=registry-nonfips",
	})
	if err != nil {
		t.Fatalf("failed to list registry-nonfips pods: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatal("no registry-nonfips pods found for log verification")
	}
	for _, pod := range pods.Items {
		req := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{
			Container: "nginx",
			TailLines: int64Ptr(50),
		})
		stream, err := req.Stream(logCtx)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(stream)
		stream.Close()
		logs := string(body)
		t.Logf("registry-nonfips nginx logs:\n%s", logs)
		if strings.Contains(logs, "no shared cipher") {
			t.Log("Verified: FIPS node rejected non-FIPS cipher suite (no shared cipher)")
			return
		}
	}
	t.Fatal("Expected 'no shared cipher' in registry-nonfips nginx logs but not found")
}

func TestFIPSTLS(t *testing.T) {
	fipsPull := features.New("fips-tls-pull").
		WithLabel("suite", "fips").
		Assess("Pull from FIPS-cipher registry succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pull-fips", Namespace: "default"},
			}
			err := wait.For(
				conditions.New(cfg.Client().Resources()).PodPhaseMatch(pod, v1.PodSucceeded),
				wait.WithContext(ctx),
				wait.WithTimeout(pullTimeout),
			)
			if err != nil {
				t.Fatalf("test-pull-fips pod did not succeed: %v", err)
			}
			t.Log("FIPS TLS pull succeeded as expected")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cfg.Client().Resources().Delete(ctx, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pull-fips", Namespace: "default"},
			})
			return ctx
		}).
		Feature()

	nonfipsPull := features.New("nonfips-tls-pull").
		WithLabel("suite", "fips").
		Assess("Pull from non-FIPS-cipher registry fails on FIPS node", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pull-nonfips", Namespace: "default"},
			}
			// Poll for ImagePullBackOff/ErrImagePull — pod won't reach PodFailed phase
			deadline := time.Now().Add(rejectTimeout)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					t.Fatalf("context cancelled while waiting for ImagePullBackOff: %v", ctx.Err())
				default:
				}
				err := cfg.Client().Resources().Get(ctx, "test-pull-nonfips", "default", pod)
				if err != nil {
					t.Fatalf("failed to get test-pull-nonfips pod: %v", err)
				}
				// #1: Log pod status during polling
				t.Logf("Polling test-pull-nonfips: Phase=%s", pod.Status.Phase)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil {
						t.Logf("  Container %s: Waiting (Reason=%s)", cs.Name, cs.State.Waiting.Reason)
					} else if cs.State.Running != nil {
						t.Logf("  Container %s: Running", cs.Name)
					} else if cs.State.Terminated != nil {
						t.Logf("  Container %s: Terminated (Reason=%s)", cs.Name, cs.State.Terminated.Reason)
					}
				}
				// #2: Detect unexpected success
				if pod.Status.Phase == v1.PodSucceeded {
					t.Fatal("test-pull-nonfips pod succeeded — expected ImagePullBackOff. Is this a FIPS node?")
				}
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Running != nil && cs.Ready {
						t.Fatal("test-pull-nonfips container is running — image pull succeeded. Is this a FIPS node?")
					}
					if cs.State.Waiting != nil && (cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
						verifyNonfipsCipherRejection(ctx, t, cfg)
						t.Log("Non-FIPS TLS pull correctly rejected (ImagePullBackOff)")
						return ctx
					}
				}
				time.Sleep(pollInterval)
			}
			t.Fatal("test-pull-nonfips did not reach ImagePullBackOff within timeout")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cfg.Client().Resources().Delete(ctx, &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pull-nonfips", Namespace: "default"},
			})
			return ctx
		}).
		Feature()

	testenv.Test(t, fipsPull, nonfipsPull)
}
