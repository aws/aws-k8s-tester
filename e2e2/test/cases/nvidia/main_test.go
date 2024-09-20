package nvidia

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	fwext "github.com/aws/aws-k8s-tester/e2e2/internal/framework_extensions"
	"github.com/aws/aws-k8s-tester/e2e2/internal/metric"
	"github.com/aws/aws-k8s-tester/e2e2/internal/utils"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv         env.Environment
	awsCfg          aws.Config
	metricManager   *metric.MetricManager
	nodeType        *string
	efaEnabled      *bool
	nvidiaTestImage *string
	nodeCount       int
	gpuPerNode      int
	efaPerNode      int
	ampMetricUrl    *string
)

var (
	//go:embed manifests/nvidia-device-plugin.yaml
	nvidiaDevicePluginManifest []byte
	//go:embed manifests/mpi-operator.yaml
	mpiOperatorManifest []byte
	//go:embed manifests/efa-device-plugin.yaml
	efaDevicePluginManifest []byte
)

func TestMain(m *testing.M) {
	nodeType = flag.String("nodeType", "", "node type for the tests")
	nvidiaTestImage = flag.String("nvidiaTestImage", "", "nccl test image for nccl tests")
	efaEnabled = flag.Bool("efaEnabled", false, "enable efa tests")
	ampMetricUrl = flag.String("ampMetricUrl", "", "amp metric url, if set, test will emit metric to the amp")
	ampMetricRoleArn := flag.String("ampMetricRoleArn", "", "amp metric role arn, if not set, default role will be used")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	awsCfg, err = utils.NewConfig()
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Minute)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	// all NVIDIA tests require the device plugin and MPI operator
	manifests := [][]byte{
		nvidiaDevicePluginManifest,
		mpiOperatorManifest,
	}

	testenv.Setup(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.ApplyManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			ds := appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "nvidia-device-plugin-daemonset", Namespace: "kube-system"},
			}
			err := wait.For(fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
				wait.WithContext(ctx))
			if err != nil {
				return ctx, fmt.Errorf("failed to deploy nvidia-device-plugin: %v", err)
			}
			if *efaEnabled {
				err := fwext.ApplyManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
				if err != nil {
					return ctx, err
				}
				ds := appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: "aws-efa-k8s-device-plugin-daemonset", Namespace: "kube-system"},
				}
				err = wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).DaemonSetReady(&ds),
					wait.WithContext(ctx))
				if err != nil {
					return ctx, fmt.Errorf("failed to deploy efa-device-plugin: %v", err)
				}
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			dep := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "mpi-operator", Namespace: "mpi-operator"},
			}
			err := wait.For(conditions.New(config.Client().Resources()).DeploymentConditionMatch(&dep, appsv1.DeploymentAvailable, v1.ConditionTrue),
				wait.WithContext(ctx))
			if err != nil {
				return ctx, fmt.Errorf("failed to deploy mpi-operator: %v", err)
			}
			return ctx, nil
		},
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			clientset, err := kubernetes.NewForConfig(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				return ctx, err
			}

			singleNodeType := true
			for i := 1; i < len(nodes.Items)-1; i++ {
				if nodes.Items[i].Labels["node.kubernetes.io/instance-type"] != nodes.Items[i-1].Labels["node.kubernetes.io/instance-type"] {
					singleNodeType = false
				}
			}
			if !singleNodeType {
				return ctx, fmt.Errorf("Node types are not the same, all node types must be the same in the cluster")
			}
			if *nodeType != "" {
				for _, v := range nodes.Items {
					if v.Labels["node.kubernetes.io/instance-type"] == *nodeType {
						nodeCount++
						gpu := v.Status.Capacity["nvidia.com/gpu"]
						gpuPerNode = int(gpu.Value())
						efa := v.Status.Capacity["vpc.amazonaws.com/efa"]
						efaPerNode = int(efa.Value())
					}
				}
			} else {
				log.Printf("No node type specified. Using the node type %s in the node groups.", nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
				nodeType = aws.String(nodes.Items[0].Labels["node.kubernetes.io/instance-type"])
				nodeCount = len(nodes.Items)
				gpu := nodes.Items[0].Status.Capacity["nvidia.com/gpu"]
				gpuPerNode = int(gpu.Value())
				efa := nodes.Items[0].Status.Capacity["vpc.amazonaws.com/efa"]
				efaPerNode = int(efa.Value())
			}
			if ampMetricUrl != nil && *ampMetricUrl != "" {
				log.Printf("AMP url is set to %s", *ampMetricUrl)
				metadataLabel, err := getMetadataLabel(ctx, cfg)
				if err != nil {
					return ctx, err
				}
				metricManager = metric.NewMetricManager(metadataLabel, awsCfg, *ampMetricUrl, *ampMetricRoleArn)
			}
			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			err := fwext.DeleteManifests(cfg.Client().RESTConfig(), efaDevicePluginManifest)
			if err != nil {
				return ctx, err
			}
			slices.Reverse(manifests)
			err = fwext.DeleteManifests(config.Client().RESTConfig(), manifests...)
			if err != nil {
				return ctx, err
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
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
		fmt.Printf(" error in discoveryClient %v", err)
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

	// Construct the metadata labels
	metadataLabels := map[string]string{
		"instance_type":      *nodeType,
		"node_count":         fmt.Sprintf("%d", nodeCount),
		"kubernetes_version": k8sVersion,
		"efa_enabled":        fmt.Sprintf("%t", *efaEnabled),
		"efa_count":          fmt.Sprintf("%d", efaPerNode),
		"ami_id":             amiId,
		"os_type":            osType,
	}

	// Create a job to fetch the logs of meta info
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "metadata-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:            "metadata-job",
							Image:           *nvidiaTestImage,
							ImagePullPolicy: v1.PullAlways,
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									"nvidia.com/gpu":        node.Status.Capacity["nvidia.com/gpu"],
									"vpc.amazonaws.com/efa": node.Status.Capacity["vpc.amazonaws.com/efa"],
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the job
	_, err = clientset.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata job: %v", err)
	}

	// Wait for the job to complete
	err = wait.For(fwext.NewConditionExtension(cfg.Client().Resources()).JobSucceeded(job),
		wait.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to wait for metadata job: %v", err)
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
