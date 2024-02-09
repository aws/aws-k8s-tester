package eksapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"k8s.io/klog/v2"
)

type AddonManager struct {
	clients *awsClients
}

func NewAddonManager(clients *awsClients) *AddonManager {
	return &AddonManager{
		clients: clients,
	}
}

func (m *AddonManager) createAddons(infra *Infrastructure, cluster *Cluster, opts *deployerOptions) error {
	for _, addon := range opts.Addons {
		addonParts := strings.Split(addon, ":")
		if len(addonParts) != 2 {
			return fmt.Errorf("invalid addon: %s", addon)
		}
		name := addonParts[0]
		version := addonParts[1]
		klog.Infof("resolving addon %s version: %s", name, version)
		resolvedVersion, err := m.resolveAddonVersion(name, version, opts.KubernetesVersion)
		if err != nil {
			return err
		}
		klog.Infof("creating addon %s version: %s", name, resolvedVersion)
		input := eks.CreateAddonInput{
			AddonName:    aws.String(name),
			AddonVersion: aws.String(resolvedVersion),
			ClusterName:  aws.String(cluster.name),
		}
		_, err = m.clients.EKS().CreateAddon(context.TODO(), &input)
		if err != nil {
			return fmt.Errorf("failed to create addon: %v", err)
		}
	}
	return nil
}

func (m *AddonManager) resolveAddonVersion(name string, versionMarker string, kubernetesVersion string) (string, error) {
	input := eks.DescribeAddonVersionsInput{
		AddonName:         aws.String(name),
		KubernetesVersion: aws.String(kubernetesVersion),
	}
	descOutput, err := m.clients.EKS().DescribeAddonVersions(context.TODO(), &input)
	if err != nil {
		return "", err
	}
	for _, addon := range descOutput.Addons {
		for _, versionInfo := range addon.AddonVersions {
			switch versionMarker {
			case "latest":
				return *versionInfo.AddonVersion, nil
			case "default":
				for _, compatibility := range versionInfo.Compatibilities {
					if compatibility.DefaultVersion {
						return *versionInfo.AddonVersion, nil
					}
				}
			default:
				if *versionInfo.AddonVersion == versionMarker {
					return *versionInfo.AddonVersion, nil
				}
			}
		}
	}
	return "", fmt.Errorf("failed to resolve addon version: %s=%s", name, versionMarker)
}
