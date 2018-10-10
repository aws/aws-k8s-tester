package prow

import "strings"

const (
	// ProviderAWS is AWS cloud provider.
	ProviderAWS = "AWS"
	// ProviderGCP is Google cloud provider.
	ProviderGCP = "GCP"
	// ProviderNotCategorized is the provider type that is not categorized yet.
	ProviderNotCategorized = "Not-Categorized"
)

func toProvider(name, id string) string {
	if strings.Contains(name, "GoogleCloudPlatform") {
		return ProviderGCP
	}
	if id == "pull-kubernetes-multicluster-ingress-test" {
		return ProviderGCP
	}
	switch {
	case strings.Contains(id, "-gci"),
		strings.Contains(id, "-gcp"),
		strings.Contains(id, "-gce"),
		strings.Contains(id, "-gke"):
		return ProviderGCP
	case strings.Contains(id, "-directconnect"),
		strings.Contains(id, "-aws"),
		strings.Contains(id, "-eks"):
		return ProviderAWS
	default:
		return ProviderNotCategorized
	}
}
