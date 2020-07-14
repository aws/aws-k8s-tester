package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployerConfiguration represents a test execution of aws-k8s-tester
type DeployerConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeployerConfigurationSpec   `json:"spec,omitempty"`
	Status DeployerConfigurationStatus `json:"status,omitempty"`
}

// DeployerConfigurationSpec contains all test configuration parameters
type DeployerConfigurationSpec struct {
	ClusterAutoscaler     *ClusterAutoscalerSpec     `json:"ca,omitempty"`
	ManagedNodeGroups     *ManagedNodeGroupsSpec     `json:"mng,omitempty"`
	IAMRoleServiceAccount *IAMRoleServiceAccountSpec `json:"irsa,omitempty"`
}

// DeployerConfigurationStatus contains all test configuration parameters
type DeployerConfigurationStatus struct {
	ClusterAutoscaler     *ClusterAutoscalerStatus     `json:"ca,omitempty"`
	ManagedNodeGroups     *ManagedNodeGroupStatus      `json:"mng,omitempty"`
	IAMRoleServiceAccount *IAMRoleServiceAccountStatus `json:"irsa,omitempty"`
}

// ManagedNodeGroupsSpec defines configuration parameters for the addon
type ManagedNodeGroupsSpec struct{}

// ManagedNodeGroupStatus contains the current status of the addon
type ManagedNodeGroupStatus struct {
	AddonStatus `json:",inline"`
}

// IAMRoleServiceAccountSpec defines configuration parameters for the addon
type IAMRoleServiceAccountSpec struct{}

// IAMRoleServiceAccountStatus contains the current status of the addon
type IAMRoleServiceAccountStatus struct {
	AddonStatus `json:",inline"`
}

// ClusterAutoscalerSpec defines configuration parameters for the addon
type ClusterAutoscalerSpec struct{}

// ClusterAutoscalerStatus contains the current status of the addon
type ClusterAutoscalerStatus struct {
	AddonStatus `json:",inline"`
}

// AddonStatus contains shared status fields for all addons
type AddonStatus struct {
	Installed bool `json:"installed"`
}
