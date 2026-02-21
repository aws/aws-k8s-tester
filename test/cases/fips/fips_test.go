//go:build e2e

package fips

import (
	"context"
	_ "embed"
	"io"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	// FIPS region manifests (with ecr-creds secret)
	//go:embed manifests/registry-fips.yaml
	registryFIPSManifest []byte
	//go:embed manifests/registry-nonfips.yaml
	registryNonFIPSManifest []byte

	// Non-FIPS region manifests (public image, no secret)
	//go:embed manifests/registry-fips-nonfipsregion.yaml
	registryFIPSNonFIPSRegionManifest []byte
	//go:embed manifests/registry-nonfips-nonfipsregion.yaml
	registryNonFIPSNonFIPSRegionManifest []byte

	//go:embed manifests/test-pods.yaml
	testPodsManifest []byte
)

func verifyNonfipsCipherRejection(t *testing.T, cfg *envconf.Config) {
	t.Helper()
	clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatalf("could not create clientset for log verification: %v", err)
	}
	pods, err := clientset.CoreV1().Pods("default").List(context.TODO(), metav1.ListOptions{
		LabelSelector: "name=registry-nonfips",
	})
	if err != nil || len(pods.Items) == 0 {
		t.Fatal("could not find registry-nonfips pods for log verification")
	}
	for _, pod := range pods.Items {
		req := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{
			Container: "nginx",
			TailLines: int64Ptr(50),
		})
		stream, err := req.Stream(context.TODO())
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

func int64Ptr(i int64) *int64 { return &i }

func TestFIPSTLS(t *testing.T) {
	fipsPull := features.New("fips-tls-pull").
		WithLabel("suite", "fips").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Deploying test pods")
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), testPodsManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Pull from FIPS-cipher registry succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			pod := &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pull-fips", Namespace: "default"},
			}
			err := wait.For(
				conditions.New(cfg.Client().Resources()).PodPhaseMatch(pod, v1.PodSucceeded),
				wait.WithContext(ctx),
				wait.WithTimeout(5*time.Minute),
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
			deadline := time.Now().Add(2 * time.Minute)
			for time.Now().Before(deadline) {
				err := cfg.Client().Resources().Get(ctx, "test-pull-nonfips", "default", pod)
				if err != nil {
					t.Fatalf("failed to get test-pull-nonfips pod: %v", err)
				}
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil && (cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
						verifyNonfipsCipherRejection(t, cfg)
						t.Log("Non-FIPS TLS pull correctly rejected (ImagePullBackOff)")
						return ctx
					}
				}
				time.Sleep(5 * time.Second)
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
