package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
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
	instanceSupportsRdmaRead = []string{"p5.48xlarge", "p4d.24xlarge", "p4de.24xlarge"}
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
}

func TestMPIJobPytorchTraining(t *testing.T) {
	singleNode := features.New("single-node").
		WithLabel("suite", "nvidia").
		WithLabel("hardware", "gpu").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := fwext.ApplyManifests(cfg.Client().RESTConfig(), mpiJobPytorchTrainingSingleNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
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
			err := wait.For(fwext.NewConditionExtension(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}
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
		WithLabel("suite", "nccl").
		WithLabel("hardware", "gpu").
		WithLabel("hardware", "efa").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			if *nvidiaTestImage == "" {
				t.Fatal(fmt.Errorf("nvidiaTestImage must be set to run unit test, use https://github.com/aws/aws-k8s-tester/blob/main/e2e2/test/images/nvidia/Dockerfile to build the image and -nvidiaTestImage to set the image url"))
			}
			renderedMpiJobNcclTestMultiNodeManifest, err := fwext.RenderManifests(mpiJobNcclTestMultiNodeManifest, ncclTestManifestTplVars{
				// one of the nodes will be used for the master pod
				WorkerNodeCount:     nodeCount,
				WorkerNodeGpuCount:  nodeCount * gpuPerNode,
				GpuPerNode:          gpuPerNode,
				NvidiaTestImage:     *nvidiaTestImage,
				EfaInterfacePerNode: efaPerNode,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedMpiJobNcclTestMultiNodeManifest)
			if err != nil {
				t.Fatal(err)
			}
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
			err := wait.For(conditions.New(rsrc).ResourceMatch(&j, mpiJobSucceeded),
				wait.WithContext(ctx))
			if err != nil {
				t.Fatal(err)
			}

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

			// emit metrics
			if ampMetricUrl != nil && *ampMetricUrl != "" {
				t.Log("Emitting nccl test metrics to AMP")
				busBandwidth, err := getNcclTestBusBandwidth(log)
				err = metricManager.PushMetricsToAMP("nccl_average_bandwidth_gbps", "Average NCCL bandwidth in Gigabytes per second.", busBandwidth)
				if err != nil {
					t.Fatal(err)
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

// getNcclTestBusBandwidth extracts the bus bandwidth value from the given log string and returns it as a float64
func getNcclTestBusBandwidth(log string) (float64, error) {
	// Define the regular expression to match the bus bandwidth number
	re := regexp.MustCompile(`# Avg bus bandwidth\s*:\s*([0-9.]+)`)

	// Find the first match
	match := re.FindStringSubmatch(log)

	// Check if a match is found
	if len(match) < 2 {
		return 0, fmt.Errorf("no bandwidth value found in the log")
	}

	// Convert the extracted string to a float64
	bandwidth, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse bandwidth value: %v", err)
	}

	// Return the extracted and parsed float64 value
	return bandwidth, nil
}
