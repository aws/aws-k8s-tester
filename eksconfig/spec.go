package eksconfig

// Spec contains specs for addons
type Spec struct {
	// ClusterAutoscaler defines the addon's spec
	ClusterAutoscaler *ClusterAutoscalerSpec `json:"clusterAutoscaler,omitempty"`
	// Overprovisioning defines the addon's spec
	Overprovisioning *OverprovisioningSpec `json:"overprovisioning,omitempty"`
	// MetricsServer defines the addon's spec
	MetricsServer *MetricsServerSpec `json:"metricsServer,omitempty"`
	// ClusterLoader2 defines the addon's spec
	ClusterLoader *ClusterLoaderSpec `json:"clusterLoader,omitempty"`
}
