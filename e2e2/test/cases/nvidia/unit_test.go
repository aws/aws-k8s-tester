package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	//go:embed manifests/job-unit-test-single-node.yaml
	jobUnitTestSingleNodeManifest         []byte
	renderedJobUnitTestSingleNodeManifest []byte
	//go:embed manifests/job-hpc-benchmarks.yaml
	jobHpcBenchmarksSingleNodeManifest         []byte
	renderedJobHpcBenchmarksSingleNodeManifest []byte
)

type unitTestManifestTplVars struct {
	NvidiaTestImage string
}

type hpcTestManifestTplVars struct {
	GpuPerNode int
}

func TestSingleNodeUnitTest(t *testing.T) {
	unitTest := features.New("unit-test").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *nvidiaTestImage == "" {
				t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run unit test, use https://github.com/aws/aws-k8s-tester/blob/main/e2e2/test/images/nvidia/Dockerfile to build the image and -nvidiaTestImage to set the image url"))
			}
			var err error
			renderedJobUnitTestSingleNodeManifest, err = fwext.RenderManifests(jobUnitTestSingleNodeManifest, unitTestManifestTplVars{
				NvidiaTestImage: *nvidiaTestImage,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedJobUnitTestSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("Unit test Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "unit-test-job", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "unit-test-job", Namespace: "default"},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for unit-test-job:")
			t.Log(log)
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedJobUnitTestSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	hpcTest := features.New("hpc-benckmarks").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			var err error
			renderedJobHpcBenchmarksSingleNodeManifest, err = fwext.RenderManifests(jobHpcBenchmarksSingleNodeManifest, hpcTestManifestTplVars{
				GpuPerNode: gpuPerNode,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedJobHpcBenchmarksSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("HPC test Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "hpc-benckmarks-job", Namespace: "default"},
			}
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "hpc-benckmarks-job", Namespace: "default"},
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for hpc-benckmarks-job:")
			t.Log(log)
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedJobHpcBenchmarksSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, unitTest, hpcTest)
}
