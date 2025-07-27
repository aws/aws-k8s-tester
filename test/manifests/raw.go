package manifests

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"strings"
	"time"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	//go:embed assets/nvidia-device-plugin.yaml
	NvidiaDevicePluginManifest []byte
	//go:embed assets/mpi-operator.yaml
	MpiOperatorManifest []byte

	//go:embed assets/efa-device-plugin.yaml
	EfaDevicePluginManifest []byte

	//go:embed assets/k8s-neuron-device-plugin-rbac.yml
	NeuronDevicePluginRbacManifest []byte
	//go:embed assets/k8s-neuron-device-plugin.yml
	NeuronDevicePluginManifest []byte

	//go:embed assets/dcgm-exporter.yaml
	DCGMExporterManifest []byte

	//go:embed assets/cloudwatch-agent.yaml
	CloudWatchAgentManifest []byte
)

// GetMetricDimensions parses CW metric dimensions from key=value pairs
func GetMetricDimensions(metricDimensions []string) (map[string]string, error) {
	dimensions := make(map[string]string)
	for _, pair := range metricDimensions {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid dimension: %q", pair)
		}
		key, value := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("invalid dimension: %q", pair)
		}
		dimensions[key] = value
	}
	return dimensions, nil
}

// RenderCloudWatchAgentManifest renders the CloudWatch Agent manifest with dynamic dimensions
func RenderCloudWatchAgentManifest(metricDimensionsSlice []string) ([]byte, error) {
	metricDimensions, err := GetMetricDimensions(metricDimensionsSlice)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metric dimensions: %w", err)
	}
	renderedManifest, err := fwext.RenderManifests(CloudWatchAgentManifest, map[string]interface{}{"MetricDimensions": metricDimensions})
	if err != nil {
		return nil, fmt.Errorf("failed to render CloudWatch Agent manifest: %w", err)
	}
	return renderedManifest, nil
}

// DeployDaemonSet returns a function to deploy and wait for a DaemonSet to be ready
func DeployDaemonSet(name, namespace string) env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		log.Printf("Waiting for %s daemonset to be ready.", name)
		daemonset := appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		}
		err := wait.For(
			fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&daemonset),
			wait.WithTimeout(5*time.Minute),
		)
		if err != nil {
			return ctx, fmt.Errorf("%s daemonset is not ready: %w", name, err)
		}
		log.Printf("%s daemonset is ready.", name)
		return ctx, nil
	}
}
