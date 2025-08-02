package manifests

import (
	"html/template"
	"strings"

	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
)

// RenderCloudWatchAgentManifest renders the CloudWatch Agent manifest with dynamic dimensions
func RenderCloudWatchAgentManifest(metricDimensions map[string]string) ([]byte, error) {
	var keys []string
	for key := range metricDimensions {
		keys = append(keys, `"`+key+`"`)
	}
	dimensionsStr := strings.Join(keys, ", ")
	return fwext.RenderManifests(cloudWatchAgentManifestTemplate, map[string]interface{}{
		"MetricDimensions": metricDimensions,
		"DimensionKeys":    template.HTML(dimensionsStr),
	})
}
