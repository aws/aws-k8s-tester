package eksconfig

// Spec contains specs for addons
type Spec struct {
	// ClusterAutoscaler defines the addon's spec
	ClusterAutoscaler *ClusterAutoscalerSpec `json:"clusterAutoscaler,omitempty"`
	// Overprovisioning defines the addon's spec
	Overprovisioning *OverprovisioningSpec `json:"overprovisioning,omitempty"`
}
