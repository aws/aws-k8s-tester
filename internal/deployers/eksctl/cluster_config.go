package eksctl

import (
	"fmt"

	eksctl_api "github.com/weaveworks/eksctl/pkg/apis/eksctl.io/v1alpha5"
	"k8s.io/klog"
	"sigs.k8s.io/yaml"
)

// CreateClusterConfig constructs an eksctl_api.ClusterConfig object based on UpOptions.
// This function replaces the string-based template rendering.
func (d *deployer) CreateClusterConfig() (*eksctl_api.ClusterConfig, error) {
	d.initClusterName()

	cfg := eksctl_api.NewClusterConfig()
	// Metadata
	cfg.Metadata.Name = d.clusterName
	cfg.Metadata.Region = d.Region
	cfg.Metadata.Version = d.KubernetesVersion
	// IAM
	cfg.IAM.WithOIDC = &d.WithOIDC

	amiFamily := d.AMIFamily
	if amiFamily == "" {
		amiFamily = eksctl_api.NodeImageFamilyAmazonLinux2
	}
	nodeGroupName := d.NodegroupName
	if nodeGroupName == "" {
		nodeGroupName = "ng-1"
	}
	// Create node group or managed node group (MNG)
	if d.UseUnmanagedNodegroup {
		ng := cfg.NewNodeGroup()
		// TODO: update this when we add support for SSH.
		ng.SSH = nil
		ng.AMIFamily = amiFamily
		ng.Name = nodeGroupName
		if len(d.InstanceTypes) > 0 {
			ng.InstanceType = d.InstanceTypes[0]
		}
		if d.Nodes >= 0 {
			ng.MinSize = &d.Nodes
			ng.MaxSize = &d.Nodes
			ng.DesiredCapacity = &d.Nodes
		}
		if d.VolumeSize >= 0 {
			ng.VolumeSize = &d.VolumeSize
		}
		ng.PrivateNetworking = d.PrivateNetworking
		ng.EFAEnabled = &d.EFAEnabled
		if len(d.AvailabilityZones) > 0 {
			ng.AvailabilityZones = d.AvailabilityZones
		}
		if d.AMI != "" && amiFamily == eksctl_api.NodeImageFamilyAmazonLinux2 {
			bootstrapCommand := fmt.Sprintf(`#!/bin/bash
source /var/lib/cloud/scripts/eksctl/bootstrap.helper.sh
/etc/eks/bootstrap.sh %s --kubelet-extra-args "--node-labels=${NODE_LABELS}"`, d.clusterName)
			ng.OverrideBootstrapCommand = &bootstrapCommand
		}
	} else {
		// Create managed node group
		mng := eksctl_api.NewManagedNodeGroup()
		cfg.ManagedNodeGroups = append(cfg.ManagedNodeGroups, mng)
		// TODO: update this when we add support for SSH.
		mng.SSH = nil
		mng.AMIFamily = amiFamily
		mng.Name = nodeGroupName
		mng.InstanceTypes = d.InstanceTypes
		if d.Nodes >= 0 {
			mng.MinSize = &d.Nodes
			mng.MaxSize = &d.Nodes
			mng.DesiredCapacity = &d.Nodes
		}
		if d.VolumeSize >= 0 {
			mng.VolumeSize = &d.VolumeSize
		}
		mng.PrivateNetworking = d.PrivateNetworking
		mng.EFAEnabled = &d.EFAEnabled
		if len(d.AvailabilityZones) > 0 {
			mng.AvailabilityZones = d.AvailabilityZones
		}
		if d.AMI != "" && amiFamily == eksctl_api.NodeImageFamilyAmazonLinux2 {
			bootstrapCommand := fmt.Sprintf(`#!/bin/bash
source /var/lib/cloud/scripts/eksctl/bootstrap.helper.sh
/etc/eks/bootstrap.sh %s --kubelet-extra-args "--node-labels=${NODE_LABELS}"`, d.clusterName)
			mng.OverrideBootstrapCommand = &bootstrapCommand
		} else if d.AMI != "" && amiFamily == eksctl_api.NodeImageFamilyBottlerocket {
			mng.AMI = d.AMI
		}
	}
	return cfg, nil
}

type clusterConfigTemplateParams struct {
	UpOptions
	ClusterName string
	Region      string
}

func (d *deployer) RenderClusterConfig() ([]byte, error) {

	cfg, err := d.CreateClusterConfig()
	if err != nil {
		klog.Errorf("failed to create ClusterConfig with the deployer: %v", err)
	}
	klog.Infof("rendering cluster config yaml based on the ClusterConfig: %v", cfg)
	return yaml.Marshal(cfg)
}
