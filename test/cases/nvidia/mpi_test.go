package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), mpiJobPytorchTrainingSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Manifest applied successfully")
			return ctx
		}).
		Assess("MPIJob succeeds", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			j := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "pytorch-training-single-node", Namespace: "default"},
			}
			t.Log("Waiting for single node job to complete")
			err := wait.For(fwext.NewConditionExtension(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Single node job completed")
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "pytorch-training-single-node", Namespace: "default"},
			})
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
			rsrc := cfg.Client().Resources()
			if err := kubeflowv2beta1.AddToScheme(rsrc.GetScheme()); err != nil {
				t.Fatal(err)
			}
			j := kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-node-nccl-test", Namespace: "default"},
			}
			t.Log("Waiting for multi node job to complete")
			err := wait.For(conditions.New(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Multi node job completed")

			// Verify GPU Direct RDMA is used on P4/P5
			log, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), &kubeflowv2beta1.MPIJob{
				ObjectMeta: metav1.ObjectMeta{Name: "multi-node-nccl-test", Namespace: "default"},
			})
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

func mpiJobSucceeded(obj k8s.Object) bool {
	j := obj.(*kubeflowv2beta1.MPIJob)
	for _, c := range j.Status.Conditions {
		if c.Type == kubeflowv2beta1.JobSucceeded {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}
