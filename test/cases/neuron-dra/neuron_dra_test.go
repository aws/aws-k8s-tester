//go:build e2e

package neuron_dra

import (
	"context"
	"embed"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/internal/e2e/mpijobs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

//go:embed testcases
var embeddedTestCases embed.FS

const (
	// negativeTestTimeout is the duration to wait before checking that a
	// negative test case's worker pods are still Pending.
	negativeTestTimeout = 1 * time.Minute
	// positiveTestTimeout is the duration to wait for an MPIJob to succeed.
	positiveTestTimeout = 20 * time.Minute
)

func TestNeuronDRAMultiNode(t *testing.T) {
	topo, err := GetTopologyForNodeType(*nodeType)
	if err != nil {
		t.Fatalf("resolving topology for %s: %v", *nodeType, err)
	}

	rctDir := filepath.Join("rcts", topo.RCTSubDir)
	rctIndex, err := loadRCTIndex(rctsFS, rctDir)
	if err != nil {
		t.Fatalf("loading RCT index from %s: %v", rctDir, err)
	}

	tcDir := filepath.Join("testcases", topo.TestCaseSubDir)
	tcEntries, err := fs.ReadDir(embeddedTestCases, tcDir)
	if err != nil {
		t.Fatalf("reading test case directory %s: %v", tcDir, err)
	}

	var featureList []features.Feature
	for _, entry := range tcEntries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		tcName := strings.TrimSuffix(entry.Name(), ext)
		tcPath := filepath.Join(tcDir, entry.Name())

		tcData, err := fs.ReadFile(embeddedTestCases, tcPath)
		if err != nil {
			t.Fatalf("reading test case %s: %v", tcPath, err)
		}

		tc, err := ParseTestCaseSpec(tcData)
		if err != nil {
			t.Fatalf("parsing test case %s: %v", tcPath, err)
		}

		params, err := ComputeMPIJobParamsFromTestCase(tc, rctIndex, topo, nodeCount, *containerTestImage)
		if err != nil {
			t.Fatalf("computing MPIJob params for %s: %v", tcName, err)
		}

		renderedYAML, err := RenderMPIJobYAML(*params)
		if err != nil {
			t.Fatalf("rendering MPIJob YAML for %s: %v", tcName, err)
		}

		if tc.ExpectFailure {
			featureList = append(featureList, buildNegativeFeature(tcName, renderedYAML))
		} else {
			featureList = append(featureList, buildPositiveFeature(tcName, renderedYAML))
		}
	}

	if len(featureList) == 0 {
		t.Logf("no test cases found under %s, skipping", tcDir)
		return
	}

	testenv.Test(t, featureList...)
}

func buildPositiveFeature(name string, manifest []byte) features.Feature {
	return features.New(name).
		WithLabel("suite", "neuron-dra").
		WithLabel("type", "positive").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Logf("Applying MPIJob manifest for %s", name)
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), manifest); err != nil {
				t.Fatalf("applying MPIJob manifest: %v", err)
			}
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			mpiJob := mpijobs.NewUnstructured("multi-node-nccom-test", "default")
			t.Log("Waiting for MPIJob to complete")
			err := wait.For(
				conditions.New(cfg.Client().Resources()).ResourceMatch(mpiJob, mpijobs.MPIJobSucceeded),
				wait.WithContext(ctx),
				wait.WithTimeout(positiveTestTimeout),
			)
			if err != nil {
				t.Errorf("MPIJob did not succeed: %v", err)
			}

			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), mpiJob)
			if err != nil {
				t.Errorf("failed to get job logs: %v", err)
			} else {
				t.Logf("Test log for %s:", name)
				t.Log(log)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), manifest); err != nil {
				t.Errorf("deleting MPIJob manifest: %v", err)
			}
			return ctx
		}).
		Feature()
}

func buildNegativeFeature(name string, manifest []byte) features.Feature {
	return features.New(name).
		WithLabel("suite", "neuron-dra").
		WithLabel("type", "negative").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Logf("Applying MPIJob manifest for negative test %s", name)
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), manifest); err != nil {
				t.Fatalf("applying MPIJob manifest: %v", err)
			}
			return ctx
		}).
		Assess("Worker pods remain Pending", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Waiting for worker pods to be Pending...")
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				t.Fatalf("creating kubernetes client: %v", err)
			}
			selector := "training.kubeflow.org/job-name=multi-node-nccom-test,training.kubeflow.org/job-role=worker"
			err = wait.For(func(ctx context.Context) (bool, error) {
				pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{LabelSelector: selector})
				if err != nil {
					return false, nil
				}
				pending := 0
				for _, pod := range pods.Items {
					if pod.Status.Phase == corev1.PodPending {
						pending++
					}
				}
				return pending >= nodeCount, nil
			}, wait.WithContext(ctx), wait.WithTimeout(negativeTestTimeout))
			if err != nil {
				t.Fatalf("expected %d worker pods in Pending state: %v", nodeCount, err)
			}
			t.Logf("All %d worker pods are Pending as expected (scheduling failure confirmed)", nodeCount)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if err := fwext.DeleteManifests(cfg.Client().RESTConfig(), manifest); err != nil {
				t.Errorf("deleting MPIJob manifest: %v", err)
			}
			return ctx
		}).
		Feature()
}
