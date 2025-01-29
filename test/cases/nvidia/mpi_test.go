//go:build e2e

package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"testing"

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
	mpiJobNcclTestMultiNodeManifest         []byte
	renderedMpiJobNcclTestMultiNodeManifest []byte
)

type ncclTestManifestTplVars struct {
	WorkerNodeCount     int
	WorkerNodeGpuCount  int
	GpuPerNode          int
	NvidiaTestImage     string
	EfaInterfacePerNode int
	MaxBytes            string
	NcclBuffSize        string
}

func TestMPIJobPytorchTraining(t *testing.T) {
	singleNode := features.New("single-node").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Log("Applying single node manifest")
			renderedSingleNodeManifest, err := fwext.RenderManifests(mpiJobPytorchTrainingSingleNodeManifest, struct {
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
				t.Fatal(err)
			}
			t.Log("Single node job completed")
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
				t.Fatalf("mpiJob in context is not unstructured: %v", job)
			}
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), u)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for pytorch-training-single-node:")
			t.Log(log)
			err = fwext.DeleteManifests(cfg.Client().RESTConfig(), mpiJobPytorchTrainingSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Feature()

	multiNode := features.New("multi-node").
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
			renderedMpiJobNcclTestMultiNodeManifest, err := fwext.RenderManifests(mpiJobNcclTestMultiNodeManifest, ncclTestManifestTplVars{
				// one of the nodes will be used for the master pod
				WorkerNodeCount:     nodeCount,
				WorkerNodeGpuCount:  nodeCount * gpuPerNode,
				GpuPerNode:          gpuPerNode,
				NvidiaTestImage:     *nvidiaTestImage,
				EfaInterfacePerNode: efaPerNode,
				MaxBytes:            maxBytes,
				NcclBuffSize:        ncclBuffSize,
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
			mpiJob := mpijobs.NewUnstructured("multi-node-nccl-test", "default")
			t.Log("Waiting for multi node job to complete")
			err := wait.For(conditions.New(cfg.Client().Resources()).ResourceMatch(mpiJob, mpijobs.MPIJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Multi node job completed")

			// Verify GPU Direct RDMA is used on P4/P5
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), mpiJob)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Test log for multi-node-nccl-test:")
			t.Log(log)
			if *efaEnabled && slices.Contains(instanceSupportsRdmaRead, *nodeType) {
				pattern := regexp.MustCompile(`\[send\] via NET/.*Libfabric/.*/GDRDMA`)
				if !pattern.MatchString(log) {
					t.Fatalf("GPU Direct RDMA is not utilized for inter-node communication in NCCL tests on instances that support GDRDMA: %s", *nodeType)
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

	testenv.Test(t, singleNode, multiNode)
}
