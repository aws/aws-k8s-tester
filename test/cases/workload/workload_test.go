//go:build e2e

package workload

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var (
	//go:embed manifests/single-node-test-workload.yaml
	workloadSingleNodeManifest         []byte
	renderedWorkloadSingleNodeManifest []byte
)

type workloadSingleNodeManifestTplVars struct {
	WorkloadTestCommand string
	WorkloadTestImage   string
	WorkloadTestName    string
}

func TestWorkload(t *testing.T) {
	singleNode := features.New("single-node").
		WithLabel("suite", "workload").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *workloadTestCommand == "" {
				t.Fatal(fmt.Errorf("workloadTestCommand must be set to run workload test"))
			}
			if *workloadTestImage == "" {
				t.Fatal(fmt.Errorf("workloadTestImage must be set to run workload test"))
			}
			if *workloadTestName == "" {
				t.Fatal(fmt.Errorf("workloadTestName must be set to run workload test"))
			}
			var err error
			renderedWorkloadSingleNodeManifest, err = fwext.RenderManifests(workloadSingleNodeManifest, workloadSingleNodeManifestTplVars{
				WorkloadTestCommand: *workloadTestCommand,
				WorkloadTestImage:   *workloadTestImage,
				WorkloadTestName:    *workloadTestName,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Applying single node manifest")
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedWorkloadSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Manifest applied successfully")
			return ctx
		}).
		Assess("Single node test Job succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: *workloadTestName, Namespace: "default"},
			}
			t.Log("Waiting for single node job to complete")
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
				wait.WithContext(ctx),
				wait.WithTimeout(time.Minute*20),
			)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: *workloadTestName, Namespace: "default"},
			})
			if err != nil {
				t.Error(err)
			} else {
				t.Log(fmt.Sprintf("Test log for %s:", *workloadTestName))
				t.Log(log)
			}
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedWorkloadSingleNodeManifest)
			if err != nil {
				t.Error(err)
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, singleNode)
}
