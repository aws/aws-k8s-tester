// Package eksdeprecate defines deprecated APIs for EKS.
package eksdeprecate

import (
	"fmt"

	apps_v1 "k8s.io/api/apps/v1"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	apps_v1beta2 "k8s.io/api/apps/v1beta2"
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	networking_v1 "k8s.io/api/networking/v1"
	policy_v1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var deprecates = map[float64]map[metav1.TypeMeta]metav1.TypeMeta{

	// https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.16.md#deprecations-and-removals
	// "kubectl convert" is deprecated
	// https://github.com/kubernetes/kubectl/issues/725
	//
	// The following APIs are no longer served by default:
	//	All resources under apps/v1beta1 and apps/v1beta2 - use apps/v1 instead
	//	daemonsets, deployments, replicasets resources under extensions/v1beta1 - use apps/v1 instead
	//	networkpolicies resources under extensions/v1beta1 - use networking.k8s.io/v1 instead
	//	podsecuritypolicies resources under extensions/v1beta1 - use policy/v1beta1 instead
	//
	// e.g.
	//	no matches for kind "DaemonSet" in version "extensions/v1beta1"
	//	no matches for kind "Deployment" in version "extensions/v1beta1"
	1.16: {
		metav1.TypeMeta{APIVersion: "apps/v1beta1", Kind: "Deployment"}:              metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		metav1.TypeMeta{APIVersion: "apps/v1beta1", Kind: "StatefulSet"}:             metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		metav1.TypeMeta{APIVersion: "apps/v1beta2", Kind: "Deployment"}:              metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		metav1.TypeMeta{APIVersion: "apps/v1beta2", Kind: "StatefulSet"}:             metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		metav1.TypeMeta{APIVersion: "extensions/v1beta1", Kind: "DaemonSet"}:         metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		metav1.TypeMeta{APIVersion: "extensions/v1beta1", Kind: "Deployment"}:        metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		metav1.TypeMeta{APIVersion: "extensions/v1beta1", Kind: "ReplicaSet"}:        metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
		metav1.TypeMeta{APIVersion: "extensions/v1beta1", Kind: "NetworkPolicy"}:     metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
		metav1.TypeMeta{APIVersion: "extensions/v1beta1", Kind: "PodSecurityPolicy"}: metav1.TypeMeta{APIVersion: "policy/v1beta1", Kind: "PodSecurityPolicy"},
	},
}

// APIs returns all APIs that need to be deprecated before upgrading to the target version.
func APIs(targetVer float64) (map[metav1.TypeMeta]metav1.TypeMeta, error) {
	v, ok := deprecates[targetVer]
	if !ok {
		return nil, fmt.Errorf("target version %.2f is not defined for upgrades", targetVer)
	}
	return v, nil
}

func ConvertAppsV1beta1ToAppsV1Deployment(obj apps_v1beta1.Deployment) (rs apps_v1.Deployment, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.DeploymentSpec{
			Replicas:                cs.Replicas,
			Selector:                cs.Selector,
			Template:                cs.Template,
			Strategy:                apps_v1.DeploymentStrategy{},
			MinReadySeconds:         cs.MinReadySeconds,
			RevisionHistoryLimit:    cs.RevisionHistoryLimit,
			Paused:                  cs.Paused,
			ProgressDeadlineSeconds: cs.ProgressDeadlineSeconds,
		},
	}
	switch cs.Strategy.Type {
	case apps_v1beta1.RecreateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RecreateDeploymentStrategyType
	case apps_v1beta1.RollingUpdateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RollingUpdateDeploymentStrategyType
	default:
		return rs, fmt.Errorf("unknown Strategy.Type %q", cs.Strategy.Type)
	}
	if cs.Strategy.RollingUpdate != nil {
		rs.Spec.Strategy.RollingUpdate = &apps_v1.RollingUpdateDeployment{}
		if cs.Strategy.RollingUpdate.MaxUnavailable != nil {
			rs.Spec.Strategy.RollingUpdate.MaxUnavailable = cs.Strategy.RollingUpdate.MaxUnavailable
		}
		if cs.Strategy.RollingUpdate.MaxSurge != nil {
			rs.Spec.Strategy.RollingUpdate.MaxSurge = cs.Strategy.RollingUpdate.MaxSurge
		}
	}
	return rs, nil
}

func ConvertAppsV1beta1ToAppsV1StatefulSet(obj apps_v1beta1.StatefulSet) (rs apps_v1.StatefulSet, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.StatefulSetSpec{
			Replicas:             cs.Replicas,
			Selector:             cs.Selector,
			Template:             cs.Template,
			VolumeClaimTemplates: cs.VolumeClaimTemplates,
			ServiceName:          cs.ServiceName,
			UpdateStrategy:       apps_v1.StatefulSetUpdateStrategy{},
			RevisionHistoryLimit: cs.RevisionHistoryLimit,
		},
	}
	switch cs.PodManagementPolicy {
	case apps_v1beta1.OrderedReadyPodManagement:
		rs.Spec.PodManagementPolicy = apps_v1.OrderedReadyPodManagement
	case apps_v1beta1.ParallelPodManagement:
		rs.Spec.PodManagementPolicy = apps_v1.ParallelPodManagement
	default:
		return rs, fmt.Errorf("unknown PodManagementPolicy %q", cs.PodManagementPolicy)
	}
	switch cs.UpdateStrategy.Type {
	case apps_v1beta1.RollingUpdateStatefulSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.RollingUpdateStatefulSetStrategyType
	case apps_v1beta1.OnDeleteStatefulSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.OnDeleteStatefulSetStrategyType
	default:
		return rs, fmt.Errorf("unknown UpdateStrategy.Type %q", cs.UpdateStrategy.Type)
	}
	if cs.UpdateStrategy.RollingUpdate != nil {
		rs.Spec.UpdateStrategy.RollingUpdate = &apps_v1.RollingUpdateStatefulSetStrategy{}
		if cs.UpdateStrategy.RollingUpdate.Partition != nil {
			rs.Spec.UpdateStrategy.RollingUpdate.Partition = cs.UpdateStrategy.RollingUpdate.Partition
		}
	}
	return rs, nil
}

func ConvertAppsV1beta2ToAppsV1Deployment(obj apps_v1beta2.Deployment) (rs apps_v1.Deployment, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:          copied.GetObjectMeta().GetName(),
			GenerateName:  copied.GetObjectMeta().GetGenerateName(),
			Namespace:     copied.GetObjectMeta().GetNamespace(),
			ClusterName:   copied.GetObjectMeta().GetClusterName(),
			Labels:        copied.GetObjectMeta().GetLabels(),
			Annotations:   copied.GetObjectMeta().GetAnnotations(),
			ManagedFields: copied.GetObjectMeta().GetManagedFields(),
		},
		Spec: apps_v1.DeploymentSpec{
			Replicas:                cs.Replicas,
			Selector:                cs.Selector,
			Template:                cs.Template,
			Strategy:                apps_v1.DeploymentStrategy{},
			MinReadySeconds:         cs.MinReadySeconds,
			RevisionHistoryLimit:    cs.RevisionHistoryLimit,
			Paused:                  cs.Paused,
			ProgressDeadlineSeconds: cs.ProgressDeadlineSeconds,
		},
	}
	switch cs.Strategy.Type {
	case apps_v1beta2.RecreateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RecreateDeploymentStrategyType
	case apps_v1beta2.RollingUpdateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RollingUpdateDeploymentStrategyType
	default:
		return rs, fmt.Errorf("unknown Strategy.Type %q", cs.Strategy.Type)
	}
	if cs.Strategy.RollingUpdate != nil {
		rs.Spec.Strategy.RollingUpdate = &apps_v1.RollingUpdateDeployment{}
		if cs.Strategy.RollingUpdate.MaxUnavailable != nil {
			rs.Spec.Strategy.RollingUpdate.MaxUnavailable = cs.Strategy.RollingUpdate.MaxUnavailable
		}
		if cs.Strategy.RollingUpdate.MaxSurge != nil {
			rs.Spec.Strategy.RollingUpdate.MaxSurge = cs.Strategy.RollingUpdate.MaxSurge
		}
	}
	return rs, nil
}

func ConvertAppsV1beta2ToAppsV1StatefulSet(obj apps_v1beta2.StatefulSet) (rs apps_v1.StatefulSet, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.StatefulSetSpec{
			Replicas:             cs.Replicas,
			Selector:             cs.Selector,
			Template:             cs.Template,
			VolumeClaimTemplates: cs.VolumeClaimTemplates,
			ServiceName:          cs.ServiceName,
			UpdateStrategy:       apps_v1.StatefulSetUpdateStrategy{},
			RevisionHistoryLimit: cs.RevisionHistoryLimit,
		},
	}
	switch cs.PodManagementPolicy {
	case apps_v1beta2.OrderedReadyPodManagement:
		rs.Spec.PodManagementPolicy = apps_v1.OrderedReadyPodManagement
	case apps_v1beta2.ParallelPodManagement:
		rs.Spec.PodManagementPolicy = apps_v1.ParallelPodManagement
	default:
		return rs, fmt.Errorf("unknown PodManagementPolicy %q", cs.PodManagementPolicy)
	}
	switch cs.UpdateStrategy.Type {
	case apps_v1beta2.RollingUpdateStatefulSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.RollingUpdateStatefulSetStrategyType
	case apps_v1beta2.OnDeleteStatefulSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.OnDeleteStatefulSetStrategyType
	default:
		return rs, fmt.Errorf("unknown UpdateStrategy.Type %q", cs.UpdateStrategy.Type)
	}
	if cs.UpdateStrategy.RollingUpdate != nil {
		rs.Spec.UpdateStrategy.RollingUpdate = &apps_v1.RollingUpdateStatefulSetStrategy{}
		if cs.UpdateStrategy.RollingUpdate.Partition != nil {
			rs.Spec.UpdateStrategy.RollingUpdate.Partition = cs.UpdateStrategy.RollingUpdate.Partition
		}
	}
	return rs, nil
}

func ConvertExtensionsV1beta1ToAppsV1DaemonSet(obj extensions_v1beta1.DaemonSet) (rs apps_v1.DaemonSet, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.DaemonSetSpec{
			Selector:             cs.Selector,
			Template:             cs.Template,
			UpdateStrategy:       apps_v1.DaemonSetUpdateStrategy{},
			MinReadySeconds:      cs.MinReadySeconds,
			RevisionHistoryLimit: cs.RevisionHistoryLimit,
		},
	}
	switch cs.UpdateStrategy.Type {
	case extensions_v1beta1.RollingUpdateDaemonSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.RollingUpdateDaemonSetStrategyType
	case extensions_v1beta1.OnDeleteDaemonSetStrategyType:
		rs.Spec.UpdateStrategy.Type = apps_v1.OnDeleteDaemonSetStrategyType
	default:
		return rs, fmt.Errorf("unknown UpdateStrategy.Type %q", cs.UpdateStrategy.Type)
	}
	if cs.UpdateStrategy.RollingUpdate != nil {
		rs.Spec.UpdateStrategy.RollingUpdate = &apps_v1.RollingUpdateDaemonSet{}
		if cs.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
			rs.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = cs.UpdateStrategy.RollingUpdate.MaxUnavailable
		}
	}
	return rs, nil
}

func ConvertExtensionsV1beta1ToAppsV1Deployment(obj extensions_v1beta1.Deployment) (rs apps_v1.Deployment, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.DeploymentSpec{
			Replicas:                cs.Replicas,
			Selector:                cs.Selector,
			Template:                cs.Template,
			Strategy:                apps_v1.DeploymentStrategy{},
			MinReadySeconds:         cs.MinReadySeconds,
			RevisionHistoryLimit:    cs.RevisionHistoryLimit,
			Paused:                  cs.Paused,
			ProgressDeadlineSeconds: cs.ProgressDeadlineSeconds,
		},
	}
	switch cs.Strategy.Type {
	case extensions_v1beta1.RecreateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RecreateDeploymentStrategyType
	case extensions_v1beta1.RollingUpdateDeploymentStrategyType:
		rs.Spec.Strategy.Type = apps_v1.RollingUpdateDeploymentStrategyType
	default:
		return rs, fmt.Errorf("unknown Strategy.Type %q", cs.Strategy.Type)
	}
	if cs.Strategy.RollingUpdate != nil {
		rs.Spec.Strategy.RollingUpdate = &apps_v1.RollingUpdateDeployment{}
		if cs.Strategy.RollingUpdate.MaxUnavailable != nil {
			rs.Spec.Strategy.RollingUpdate.MaxUnavailable = cs.Strategy.RollingUpdate.MaxUnavailable
		}
		if cs.Strategy.RollingUpdate.MaxSurge != nil {
			rs.Spec.Strategy.RollingUpdate.MaxSurge = cs.Strategy.RollingUpdate.MaxSurge
		}
	}
	return rs, nil
}

func ConvertExtensionsV1beta1ToAppsV1ReplicaSet(obj extensions_v1beta1.ReplicaSet) (rs apps_v1.ReplicaSet, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = apps_v1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "ReplicaSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: apps_v1.ReplicaSetSpec{
			Replicas:        cs.Replicas,
			MinReadySeconds: cs.MinReadySeconds,
			Selector:        cs.Selector,
			Template:        cs.Template,
		},
	}
	return rs, nil
}

func ConvertExtensionsV1beta1ToNetworkingV1NetworkPolicy(obj extensions_v1beta1.NetworkPolicy) (rs networking_v1.NetworkPolicy, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = networking_v1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "NetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: networking_v1.NetworkPolicySpec{
			PodSelector: cs.PodSelector,
			Ingress:     []networking_v1.NetworkPolicyIngressRule{},
			Egress:      []networking_v1.NetworkPolicyEgressRule{},
			PolicyTypes: []networking_v1.PolicyType{},
		},
	}

	for _, vv := range cs.Ingress {
		copiedv := vv.DeepCopy()
		cv := networking_v1.NetworkPolicyIngressRule{}
		for _, v := range copiedv.Ports {
			cv.Ports = append(cv.Ports, networking_v1.NetworkPolicyPort{
				Protocol: v.Protocol,
				Port:     v.Port,
			})
		}
		for _, v := range copiedv.From {
			cv.From = append(cv.From, networking_v1.NetworkPolicyPeer{
				PodSelector:       v.PodSelector,
				NamespaceSelector: v.NamespaceSelector,
			})
		}
		rs.Spec.Ingress = append(rs.Spec.Ingress, cv)
	}

	for _, vv := range cs.Egress {
		copiedv := vv.DeepCopy()
		cv := networking_v1.NetworkPolicyEgressRule{}
		for _, v := range copiedv.Ports {
			cv.Ports = append(cv.Ports, networking_v1.NetworkPolicyPort{
				Protocol: v.Protocol,
				Port:     v.Port,
			})
		}
		for _, v := range copiedv.To {
			cv.To = append(cv.To, networking_v1.NetworkPolicyPeer{
				PodSelector:       v.PodSelector,
				NamespaceSelector: v.NamespaceSelector,
			})
		}
		rs.Spec.Egress = append(rs.Spec.Egress, cv)
	}

	for _, vv := range cs.PolicyTypes {
		switch vv {
		case extensions_v1beta1.PolicyTypeIngress:
			rs.Spec.PolicyTypes = append(rs.Spec.PolicyTypes, networking_v1.PolicyTypeIngress)
		case extensions_v1beta1.PolicyTypeEgress:
			rs.Spec.PolicyTypes = append(rs.Spec.PolicyTypes, networking_v1.PolicyTypeEgress)
		default:
			return rs, fmt.Errorf("unknown extensions_v1beta1.PolicyType %q", vv)
		}
	}

	return rs, nil
}

func ConvertExtensionsV1beta1ToPolicyV1beta1PodSecurityPolicy(obj extensions_v1beta1.PodSecurityPolicy) (rs policy_v1beta1.PodSecurityPolicy, err error) {
	copied := obj.DeepCopy()
	cs := copied.Spec.DeepCopy()
	rs = policy_v1beta1.PodSecurityPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1beta1",
			Kind:       "PodSecurityPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       copied.GetObjectMeta().GetName(),
			GenerateName:               copied.GetObjectMeta().GetGenerateName(),
			Namespace:                  copied.GetObjectMeta().GetNamespace(),
			ClusterName:                copied.GetObjectMeta().GetClusterName(),
			Labels:                     copied.GetObjectMeta().GetLabels(),
			Annotations:                copied.GetObjectMeta().GetAnnotations(),
			ManagedFields:              copied.GetObjectMeta().GetManagedFields(),
			DeletionGracePeriodSeconds: copied.GetObjectMeta().GetDeletionGracePeriodSeconds(),
		},
		Spec: policy_v1beta1.PodSecurityPolicySpec{
			Privileged:               cs.Privileged,
			DefaultAddCapabilities:   cs.DefaultAddCapabilities,
			RequiredDropCapabilities: cs.RequiredDropCapabilities,
			AllowedCapabilities:      cs.AllowedCapabilities,
			HostNetwork:              cs.HostNetwork,
			HostPID:                  cs.HostPID,
			HostIPC:                  cs.HostIPC,

			ReadOnlyRootFilesystem:          cs.ReadOnlyRootFilesystem,
			DefaultAllowPrivilegeEscalation: cs.DefaultAllowPrivilegeEscalation,
			AllowPrivilegeEscalation:        cs.AllowPrivilegeEscalation,

			AllowedUnsafeSysctls:  cs.AllowedUnsafeSysctls,
			ForbiddenSysctls:      cs.ForbiddenSysctls,
			AllowedProcMountTypes: cs.AllowedProcMountTypes,
		},
	}

	for _, vv := range cs.Volumes {
		switch vv {
		case extensions_v1beta1.AzureFile:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.AzureFile)
		case extensions_v1beta1.Flocker:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.Flocker)
		case extensions_v1beta1.FlexVolume:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.FlexVolume)
		case extensions_v1beta1.HostPath:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.HostPath)
		case extensions_v1beta1.EmptyDir:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.EmptyDir)
		case extensions_v1beta1.GCEPersistentDisk:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.GCEPersistentDisk)
		case extensions_v1beta1.AWSElasticBlockStore:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.AWSElasticBlockStore)
		case extensions_v1beta1.GitRepo:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.GitRepo)
		case extensions_v1beta1.Secret:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.Secret)
		case extensions_v1beta1.NFS:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.NFS)
		case extensions_v1beta1.ISCSI:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.ISCSI)
		case extensions_v1beta1.Glusterfs:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.Glusterfs)
		case extensions_v1beta1.PersistentVolumeClaim:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.PersistentVolumeClaim)
		case extensions_v1beta1.RBD:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.RBD)
		case extensions_v1beta1.Cinder:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.Cinder)
		case extensions_v1beta1.CephFS:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.CephFS)
		case extensions_v1beta1.DownwardAPI:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.DownwardAPI)
		case extensions_v1beta1.FC:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.FC)
		case extensions_v1beta1.ConfigMap:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.ConfigMap)
		case extensions_v1beta1.Quobyte:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.Quobyte)
		case extensions_v1beta1.AzureDisk:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.AzureDisk)
		case extensions_v1beta1.CSI:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.CSI)
		case extensions_v1beta1.All:
			rs.Spec.Volumes = append(rs.Spec.Volumes, policy_v1beta1.All)
		default:
			return rs, fmt.Errorf("unknown Volume %q", vv)
		}
	}

	for _, vv := range cs.HostPorts {
		rs.Spec.HostPorts = append(rs.Spec.HostPorts, policy_v1beta1.HostPortRange{
			Min: vv.Min,
			Max: vv.Max,
		})
	}

	switch cs.SELinux.Rule {
	case extensions_v1beta1.SELinuxStrategyMustRunAs:
		rs.Spec.SELinux.Rule = policy_v1beta1.SELinuxStrategyMustRunAs
	case extensions_v1beta1.SELinuxStrategyRunAsAny:
		rs.Spec.SELinux.Rule = policy_v1beta1.SELinuxStrategyRunAsAny
	default:
		return rs, fmt.Errorf("unknown SELinux.Rule %q", cs.SELinux.Rule)
	}
	rs.Spec.SELinux.SELinuxOptions = cs.SELinux.SELinuxOptions

	switch cs.RunAsUser.Rule {
	case extensions_v1beta1.RunAsUserStrategyMustRunAs:
		rs.Spec.RunAsUser.Rule = policy_v1beta1.RunAsUserStrategyMustRunAs
	case extensions_v1beta1.RunAsUserStrategyMustRunAsNonRoot:
		rs.Spec.RunAsUser.Rule = policy_v1beta1.RunAsUserStrategyMustRunAsNonRoot
	case extensions_v1beta1.RunAsUserStrategyRunAsAny:
		rs.Spec.RunAsUser.Rule = policy_v1beta1.RunAsUserStrategyRunAsAny
	default:
		return rs, fmt.Errorf("unknown RunAsUser.Rule %q", cs.RunAsUser.Rule)
	}
	for _, vv := range cs.RunAsUser.Ranges {
		rs.Spec.RunAsUser.Ranges = append(rs.Spec.RunAsUser.Ranges, policy_v1beta1.IDRange{
			Min: vv.Min,
			Max: vv.Max,
		})
	}

	if cs.RunAsGroup != nil {
		switch cs.RunAsGroup.Rule {
		case extensions_v1beta1.RunAsGroupStrategyMayRunAs:
			rs.Spec.RunAsGroup.Rule = policy_v1beta1.RunAsGroupStrategyMayRunAs
		case extensions_v1beta1.RunAsGroupStrategyMustRunAs:
			rs.Spec.RunAsGroup.Rule = policy_v1beta1.RunAsGroupStrategyMustRunAs
		case extensions_v1beta1.RunAsGroupStrategyRunAsAny:
			rs.Spec.RunAsGroup.Rule = policy_v1beta1.RunAsGroupStrategyRunAsAny
		default:
			return rs, fmt.Errorf("unknown RunAsGroup.Rule %q", cs.RunAsGroup.Rule)
		}
		for _, vv := range cs.RunAsGroup.Ranges {
			rs.Spec.RunAsGroup.Ranges = append(rs.Spec.RunAsGroup.Ranges, policy_v1beta1.IDRange{
				Min: vv.Min,
				Max: vv.Max,
			})
		}
	}

	switch cs.SupplementalGroups.Rule {
	case extensions_v1beta1.SupplementalGroupsStrategyMustRunAs:
		rs.Spec.SupplementalGroups.Rule = policy_v1beta1.SupplementalGroupsStrategyMustRunAs
	case extensions_v1beta1.SupplementalGroupsStrategyRunAsAny:
		rs.Spec.SupplementalGroups.Rule = policy_v1beta1.SupplementalGroupsStrategyRunAsAny
	default:
		return rs, fmt.Errorf("unknown SupplementalGroups.Rule %q", cs.SupplementalGroups.Rule)
	}
	for _, vv := range cs.SupplementalGroups.Ranges {
		rs.Spec.SupplementalGroups.Ranges = append(rs.Spec.SupplementalGroups.Ranges, policy_v1beta1.IDRange{
			Min: vv.Min,
			Max: vv.Max,
		})
	}

	switch cs.FSGroup.Rule {
	case extensions_v1beta1.FSGroupStrategyMustRunAs:
		rs.Spec.FSGroup.Rule = policy_v1beta1.FSGroupStrategyMustRunAs
	case extensions_v1beta1.FSGroupStrategyRunAsAny:
		rs.Spec.FSGroup.Rule = policy_v1beta1.FSGroupStrategyRunAsAny
	default:
		return rs, fmt.Errorf("unknown FSGroup.Rule %q", cs.FSGroup.Rule)
	}
	for _, vv := range cs.FSGroup.Ranges {
		rs.Spec.FSGroup.Ranges = append(rs.Spec.FSGroup.Ranges, policy_v1beta1.IDRange{
			Min: vv.Min,
			Max: vv.Max,
		})
	}

	for _, vv := range cs.AllowedHostPaths {
		rs.Spec.AllowedHostPaths = append(rs.Spec.AllowedHostPaths, policy_v1beta1.AllowedHostPath{
			PathPrefix: vv.PathPrefix,
			ReadOnly:   vv.ReadOnly,
		})
	}

	for _, vv := range cs.AllowedFlexVolumes {
		rs.Spec.AllowedFlexVolumes = append(rs.Spec.AllowedFlexVolumes, policy_v1beta1.AllowedFlexVolume{
			Driver: vv.Driver,
		})
	}

	for _, vv := range cs.AllowedCSIDrivers {
		rs.Spec.AllowedCSIDrivers = append(rs.Spec.AllowedCSIDrivers, policy_v1beta1.AllowedCSIDriver{
			Name: vv.Name,
		})
	}

	if cs.RuntimeClass != nil {
		rs.Spec.RuntimeClass = &policy_v1beta1.RuntimeClassStrategyOptions{
			AllowedRuntimeClassNames: cs.RuntimeClass.AllowedRuntimeClassNames,
			DefaultRuntimeClassName:  cs.RuntimeClass.DefaultRuntimeClassName,
		}
	}

	return rs, nil
}
