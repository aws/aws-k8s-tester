package manifests

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

// ParseMetricDimensions parses metric dimensions string into a map
func ParseMetricDimensions(metricDimensions string) map[string]string {
	result := make(map[string]string)
	if metricDimensions != "" {
		pairs := strings.Split(metricDimensions, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}
	return result
}

// RenderCloudWatchAgentManifest renders the CloudWatch Agent manifest with dynamic dimensions
func RenderCloudWatchAgentManifest(region string, metricDimensions map[string]string) ([]byte, error) {
    templateData := map[string]interface{}{
        "REGION":           region,
        "DIMENSIONS":       "",
        "RELABEL_CONFIGS": "",
    }

    if len(metricDimensions) > 0 {
        dimensions := make([]string, 0, len(metricDimensions))
        relabelConfigs := make([]string, 0, len(metricDimensions))

        for key, value := range metricDimensions {
            templateData[strings.ToUpper(key)] = value
            dimensions = append(dimensions, fmt.Sprintf(`"%s"`, key))
            relabelConfigs = append(relabelConfigs, 
                fmt.Sprintf("          - {action: replace, target_label: %s, replacement: '%s'}", 
                    key, value))
        }

        templateData["DIMENSIONS"] = strings.Join(dimensions, ", ")
        templateData["RELABEL_CONFIGS"] = strings.Join(relabelConfigs, "\n")
    }

    renderedManifest, err := fwext.RenderManifests(CloudWatchAgentManifest, templateData)
    if err != nil {
        return nil, fmt.Errorf("failed to render CloudWatch Agent manifest: %w", err)
    }
    return renderedManifest, nil
}

// GetRegionFromNodes extracts the AWS region from node labels
func GetRegionFromNodes(ctx context.Context, config *envconf.Config) (string, error) {
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(nodes.Items) > 0 {
		return nodes.Items[0].Labels["topology.kubernetes.io/region"], nil
	}
	return "", fmt.Errorf("no nodes found in the cluster")
}

/*
TODO: Consider loading region from AWS default config if AWS API calls are added to this test

import "github.com/aws/aws-sdk-go-v2/config"
...
if cfg, err := config.LoadDefaultConfig(context.TODO()); err != nil {
return "", fmt.Errorf("failed loading config, %v", err)
} else {
return cfg.Region, nil
}
*/
