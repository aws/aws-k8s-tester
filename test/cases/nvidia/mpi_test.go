//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	"github.com/aws/aws-k8s-tester/internal/e2e/mpijobs"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/strings/slices"
)

var (
	instanceSupportsRdmaRead = []string{"p5.48xlarge", "p4d.24xlarge", "p4de.24xlarge", "p5e.48xlarge", "p5en.48xlarge"}
)

var (
	//go:embed manifests/mpi-job-pytorch-training-single-node.yaml
	mpiJobPytorchTrainingSingleNodeManifest []byte
	//go:embed manifests/mpi-job-nccl-test-multi-node.yaml
	mpiJobNcclTestMultiNodeManifest []byte
)

type ncclTestManifestTplVars struct {
	WorkerNodeCount     int
	WorkerNodeGpuCount  int
	GpuPerNode          int
	NvidiaTestImage     string
	EfaInterfacePerNode int
	MaxBytes            string
	NcclBuffSize        string
	TestName            string
	JobName             string
}

func TestMPIJobPytorchTraining(t *testing.T) {
	testenv.Test(t,
		singleNode(),
		multiNode("all_reduce_perf"),
		multiNode("all_gather_perf"),
		multiNode("alltoall_perf"),
	)
}

func multiNode(testName string) features.Feature {
	var renderedMpiJobNcclTestMultiNodeManifest []byte
	jobName := strings.ReplaceAll(fmt.Sprintf("multi-node-%s", testName), "_", "-")

	return features.New(fmt.Sprintf("multi-node:%s", testName)).
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		WithLabel("hardware", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *nvidiaTestImage == "" {
				t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run unit test, use https://github.com/aws/aws-k8s-tester/blob/main/test/images/nvidia/Dockerfile to build the image and -nvidiaTestImage to set the image url"))
			}
			maxBytes := "2G"
			ncclBuffSize := "4194304"
			if slices.Contains(instanceSupportsRdmaRead, *nodeType) {
				t.Log("Instance supports RDMA")
				maxBytes = "16G"
				ncclBuffSize = "8388608"
			}
			var err error
			renderedMpiJobNcclTestMultiNodeManifest, err = fwext.RenderManifests(mpiJobNcclTestMultiNodeManifest, ncclTestManifestTplVars{
				// one of the nodes will be used for the master pod
				WorkerNodeCount:     nodeCount,
				WorkerNodeGpuCount:  nodeCount * gpuPerNode,
				GpuPerNode:          gpuPerNode,
				NvidiaTestImage:     *nvidiaTestImage,
				EfaInterfacePerNode: efaPerNode,
				MaxBytes:            maxBytes,
				NcclBuffSize:        ncclBuffSize,
				TestName:            testName,
				JobName:             jobName,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Applying multi node manifest")
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedMpiJobNcclTestMultiNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Manifest applied successfully")
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			mpiJob := mpijobs.NewUnstructured(jobName, "default")
			t.Log("Waiting for multi node job to complete")
			waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
			defer cancel()
			err := wait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(mpiJob, mpijobs.MPIJobSucceeded),
				wait.WithContext(waitCtx),
			)
			if err != nil {
				t.Error(err)
			}
			t.Logf("final mpijob resource: %v", mpiJob)
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), mpiJob)
			if err != nil {
				t.Errorf("failed to get job logs: %v", err)
			}
			t.Logf("Test log for %s:", jobName)
			t.Log(log)

			if !t.Failed() {
				t.Log("Multi node job completed")
				// Verify GPU Direct RDMA is used on P4/P5
				if *efaEnabled && slices.Contains(instanceSupportsRdmaRead, *nodeType) {
					pattern := regexp.MustCompile(`\[send\] via NET/.*Libfabric/.*/GDRDMA`)
					if !pattern.MatchString(log) {
						t.Errorf("GPU Direct RDMA is not utilized for inter-node communication in NCCL tests on instances that support GDRDMA: %s", *nodeType)
					}
				}
			}
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedMpiJobNcclTestMultiNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()
}

func singleNode() features.Feature {
	var renderedSingleNodeManifest []byte

	return features.New("single-node").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Applying single node manifest")
			var err error
			renderedSingleNodeManifest, err = fwext.RenderManifests(mpiJobPytorchTrainingSingleNodeManifest, struct {
				PytorchTestImage string
			}{
				PytorchTestImage: *pytorchImage,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Manifest applied successfully")
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			mpiJob := mpijobs.NewUnstructured("pytorch-training-single-node", "default")
			ctx = context.WithValue(ctx, "mpiJob", mpiJob)
			t.Log("Waiting for single node job to complete")
			err := wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).ResourceMatch(mpiJob, mpijobs.MPIJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Error(err)
			} else {
				t.Log("Single node job completed")
			}
			t.Logf("final mpijob resource: %v", mpiJob)
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			job := ctx.Value("mpiJob")
			if job == nil {
				// nothing to do
				return ctx
			}
			u, ok := job.(*unstructured.Unstructured)
			if !ok {
				t.Errorf("mpiJob in context is not unstructured: %v", job)
			}
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), u)
			if err != nil {
				t.Errorf("failed to get job logs: %v", err)
			}
			t.Log("Test log for pytorch-training-single-node:")
			t.Log(log)
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), renderedSingleNodeManifest)
			if err != nil {
				t.Error(err)
			}
			return ctx
		}).
		Feature()

}
