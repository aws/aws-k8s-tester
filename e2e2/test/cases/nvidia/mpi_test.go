package nvidia

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	kubeflowv2beta1 "github.com/kubeflow/mpi-operator/pkg/apis/kubeflow/v2beta1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/strings/slices"
)

var (
	instanceSupportsRdmaRead = []string{"p5.48xlarge", "p4d.24xlarge", "p4de.24xlarge"}
)

var (
	//go:embed manifests/mpi-job-pytorch-training-single-node.yaml
	mpiJobPytorchTrainingSingleNodeManifest []byte
	//go:embed manifests/mpi-job-nccl-test-multi-node.yaml
	mpiJobNcclTestMultiNodeManifest []byte
	//go:embed manifests/metadata-job.yaml
	metadataJobManifest                     []byte
	renderedMpiJobNcclTestMultiNodeManifest []byte
)

type ncclTestManifestTplVars struct {
	WorkerNodeCount     int
	WorkerNodeGpuCount  int
	GpuPerNode          int
	NvidiaTestImage     string
	EfaInterfacePerNode int
}

type metadataJobManifestTplVars struct {
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
				metadataLabel, err := getMetadataLabel(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				metric := prometheus.NewGauge(prometheus.GaugeOpts{
					Name:        "nccl_average_bandwidth_gbps",
					Help:        "Average NCCL bandwidth in Gigabytes per second.",
					ConstLabels: metadataLabel,
				})
				busBandwidth, err := getNcclTestBusBandwidth(log)
				metric.Set(busBandwidth)
				metricManager.Registry.MustRegister(metric)
				if err != nil {
					t.Fatal(err)
				}
				err = metricManager.PushMetricsToAMP()
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

func getMetadataLabel(ctx context.Context, cfg *envconf.Config) (map[string]string, error) {
	restConfig := cfg.Client().RESTConfig()
	// Initialize Kubernetes client from the provided REST config
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// List nodes in the cluster
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	// Ensure at least one node is available
	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("no nodes found in the cluster")
	}

	// Get instance type and metadata from the first node
	node := nodes.Items[0]
	osType := node.Status.NodeInfo.OSImage

	// Get k8s cluster version
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf(" error in discoveryClient %v", err)
	}
	information, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s version: %v", err)
	}
	k8sVersion := fmt.Sprintf("%s.%s", information.Major, information.Minor)

	// get ami id
	instanceId := regexp.MustCompile(`i-[a-zA-Z0-9]+`).FindString(node.Spec.ProviderID)
	// describe the instance
	ec2Client := ec2.NewFromConfig(awsCfg)
	instance, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance: %v", err)
	}
	amiId := *instance.Reservations[0].Instances[0].ImageId
	//get ami name from id
	ami, err := ec2Client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amiId},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe ami: %v", err)
	}
	amiName := ami.Images[0].Name
	// Construct the metadata labels
	metadataLabels := map[string]string{
		"instance_type":      *nodeType,
		"node_count":         fmt.Sprintf("%d", nodeCount),
		"kubernetes_version": k8sVersion,
		"efa_enabled":        fmt.Sprintf("%t", *efaEnabled),
		"efa_count":          fmt.Sprintf("%d", efaPerNode),
		"ami_id":             amiId,
		"ami_name":           *amiName,
		"os_type":            osType,
	}

	// Create a job to fetch the logs of meta info
	renderedMetadataJobManifest, err := fwext.RenderManifests(metadataJobManifest, metadataJobManifestTplVars{
		GpuPerNode:          gpuPerNode,
		NvidiaTestImage:     *nvidiaTestImage,
		EfaInterfacePerNode: efaPerNode,
	})
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}
	err = fwext.ApplyManifests(cfg.Client().RESTConfig(), renderedMetadataJobManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata job: %v", err)
	}

	// Wait for the job to complete
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "metadata-job", Namespace: "default"},
	}
	err = wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
		wait.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to wait for metadata job to complete: %v", err)
	}

	// get the logs
	logs, err := fwext.GetJobLogs(cfg.Client().RESTConfig(), job)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata job logs: %v", err)
	}

	// Define regex patterns for each version
	patterns := map[string]string{
		"efa_installer_version": `EFA Installer Version:\s*([0-9.]+)`,
		"nccl_version":          `NCCL Version:\s*([0-9.]+)`,
		"aws_ofi_nccl_version":  `AWS OFI NCCL Version:\s*([0-9.]+)`,
		"nvidia_driver_version": `NVIDIA Driver Version:\s*([0-9.]+)`,
	}
	// Extract software versions using regex from logs
	for key, pattern := range patterns {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(logs); len(match) > 1 {
			metadataLabels[key] = match[1]
		}
	}

	// cleanup the job
	deletePolicy := metav1.DeletePropagationBackground
	err = clientset.BatchV1().Jobs("default").Delete(ctx, job.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete metadata job: %v", err)
	}

	return metadataLabels, nil
}
