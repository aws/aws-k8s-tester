package manifests

import (
	_ "embed"
	"fmt"
	"strings"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
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

func RenderCloudWatchAgentManifest(kubernetesVersion, amiVariant, nodeType, region, teamIdentifier, testName string) ([]byte, error) {
	templateData := map[string]string{
		"VERSION":         strings.Replace(kubernetesVersion, ".", "", 1),
		"VARIANT":         strings.ToLower(amiVariant),
		"INSTANCE_TYPE":   nodeType,
		"REGION":          region,
		"TEAM_IDENTIFIER": teamIdentifier,
		"TEST_NAME":       testName,
	}

	renderedManifest, err := fwext.RenderManifests(CloudWatchAgentManifest, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render CloudWatch Agent manifest: %w", err)
	}
	return renderedManifest, nil
}
