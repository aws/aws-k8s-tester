//go:build e2e

package common

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/internal/e2e/mpijobs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	// NegativeTestTimeout is the duration to wait before checking that a
	// negative test case's worker pods are still Pending.
	NegativeTestTimeout = 1 * time.Minute
	// NegativeTestStabilizationTimeout is the duration to wait after pods
	// are first observed as Pending before re-checking they remain Pending.
	NegativeTestStabilizationTimeout = 2 * time.Minute
	// PositiveTestTimeout is the duration to wait for an MPIJob to succeed.
	PositiveTestTimeout = 20 * time.Minute
)

// ComputeAndRenderFunc is a callback that computes MPIJob parameters and renders
// the MPIJob YAML for a given test case. Each package provides its own implementation
// that calls its package-specific ComputeMPIJobParams and RenderMPIJobYAML functions.
type ComputeAndRenderFunc func(tc *TestCaseSpec, rctIndex map[string]*ResourceClaimTemplateSpec) (renderedYAML []byte, err error)

// BuildPositiveFeature constructs an e2e-framework Feature for a positive DRA
// test case. It applies the manifest, waits for the MPIJob to succeed, retrieves
// logs, and cleans up.
func BuildPositiveFeature(name, suiteName, mpiJobName string, manifest []byte) features.Feature {
	return features.New(name).
		WithLabel("suite", suiteName).
		WithLabel("type", "positive").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Logf("Applying MPIJob manifest for %s", name)
			if err := fwext.ApplyManifests(cfg.Client().RESTConfig(), manifest); err != nil {
				t.Fatalf("applying MPIJob manifest: %v", err)
			}
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			mpiJob := mpijobs.NewUnstructured(mpiJobName, "default")
			t.Log("Waiting for MPIJob to complete")
			err := wait.For(
				conditions.New(cfg.Client().Resources()).ResourceMatch(mpiJob, mpijobs.MPIJobSucceeded),
				wait.WithContext(ctx),
				wait.WithTimeout(PositiveTestTimeout),
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

// BuildNegativeFeature constructs an e2e-framework Feature for a negative DRA
// test case. It applies the manifest, waits for a timeout, verifies worker pods
// remain Pending, and cleans up.
func BuildNegativeFeature(name, suiteName, mpiJobName string, manifest []byte, expectedPendingCount int, clientset kubernetes.Interface) features.Feature {
	return features.New(name).
		WithLabel("suite", suiteName).
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
			selector := fmt.Sprintf("training.kubeflow.org/job-name=%s,training.kubeflow.org/job-role=worker", mpiJobName)
			listOpts := metav1.ListOptions{
				LabelSelector: selector,
				FieldSelector: "status.phase=Pending",
			}
			err := wait.For(func(ctx context.Context) (bool, error) {
				pods, err := clientset.CoreV1().Pods("default").List(ctx, listOpts)
				if err != nil {
					return false, nil
				}
				return len(pods.Items) >= expectedPendingCount, nil
			}, wait.WithContext(ctx), wait.WithTimeout(NegativeTestTimeout))
			if err != nil {
				t.Fatalf("expected %d worker pods in Pending state: %v", expectedPendingCount, err)
			}
			t.Logf("Found %d Pending worker pods, waiting %v to confirm they remain unschedulable...", expectedPendingCount, NegativeTestStabilizationTimeout)
			time.Sleep(NegativeTestStabilizationTimeout)
			pods, err := clientset.CoreV1().Pods("default").List(ctx, listOpts)
			if err != nil {
				t.Fatalf("re-checking Pending pods: %v", err)
			}
			if len(pods.Items) < expectedPendingCount {
				t.Fatalf("expected %d Pending worker pods after stabilization, but found %d", expectedPendingCount, len(pods.Items))
			}
			t.Logf("All %d worker pods are still Pending after stabilization (scheduling failure confirmed)", expectedPendingCount)
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

// DiscoverAndBuildFeatures encapsulates the common test discovery loop:
//  1. Reads test case YAML files from testCasesFS at testCaseDir
//  2. Parses each via ParseTestCaseSpec
//  3. Invokes computeAndRender to get the rendered MPIJob YAML
//  4. Builds positive or negative features based on ExpectFailure
func DiscoverAndBuildFeatures(
	testCasesFS fs.FS,
	testCaseDir string,
	rctIndex map[string]*ResourceClaimTemplateSpec,
	suiteName string,
	mpiJobName string,
	nodeCount int,
	computeAndRender ComputeAndRenderFunc,
	clientset kubernetes.Interface,
) ([]features.Feature, error) {
	entries, err := fs.ReadDir(testCasesFS, testCaseDir)
	if err != nil {
		return nil, fmt.Errorf("reading test case directory %s: %w", testCaseDir, err)
	}

	var featureList []features.Feature
	for _, entry := range entries {
		if entry.IsDir() || !IsYAMLFile(entry.Name()) {
			continue
		}

		tcName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		tcPath := filepath.Join(testCaseDir, entry.Name())

		tcData, err := fs.ReadFile(testCasesFS, tcPath)
		if err != nil {
			return nil, fmt.Errorf("reading test case %s: %w", tcPath, err)
		}

		tc, err := ParseTestCaseSpec(tcData)
		if err != nil {
			return nil, fmt.Errorf("parsing test case %s: %w", tcPath, err)
		}

		renderedYAML, err := computeAndRender(tc, rctIndex)
		if err != nil {
			return nil, fmt.Errorf("computing/rendering MPIJob for %s: %w", tcName, err)
		}

		if tc.ExpectFailure {
			featureList = append(featureList, BuildNegativeFeature(tcName, suiteName, mpiJobName, renderedYAML, nodeCount, clientset))
		} else {
			featureList = append(featureList, BuildPositiveFeature(tcName, suiteName, mpiJobName, renderedYAML))
		}
	}
	return featureList, nil
}
