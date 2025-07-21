package eksapi

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/aws/aws-k8s-tester/internal/deployers/eksapi/templates"
)

func generateUserData(cluster *Cluster, opts *deployerOptions) (string, bool, error) {
	userDataIsMimePart := true
	var t *template.Template
	switch opts.UserDataFormat {
	case "bootstrap.sh":
		t = templates.UserDataBootstrapSh
	case "nodeadm":
		// TODO: replace the YAML template with proper usage of the nodeadm API go types
		t = templates.UserDataNodeadm
	case "bottlerocket":
		t = templates.UserDataBottlerocket
		userDataIsMimePart = false
	default:
		return "", false, fmt.Errorf("unknown user data format: '%s'", opts.UserDataFormat)
	}

	kubeletFeatureGates := map[string]bool{}
	// DRA is in beta for 1.33, and so needs to be explicitly enabled.
	if opts.KubernetesVersion == "1.33" {
		kubeletFeatureGates["DynamicResourceAllocation"] = true
	}

	nodeadmFeatureGates, err := extractFeatureGates(opts.NodeadmFeatureGates)
	if err != nil {
		return "", false, err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, templates.UserDataTemplateData{
		APIServerEndpoint:    cluster.endpoint,
		CertificateAuthority: cluster.certificateAuthorityData,
		CIDR:                 cluster.cidr,
		Name:                 cluster.name,
		KubeletFeatureGates:  kubeletFeatureGates,
		NodeadmFeatureGates:  nodeadmFeatureGates,
	}); err != nil {
		return "", false, err
	}
	return buf.String(), userDataIsMimePart, nil
}

func extractFeatureGates(featureGatePairs []string) (map[string]bool, error) {
	featureGateMap := make(map[string]bool)
	for _, keyValuePair := range featureGatePairs {
		components := strings.Split(keyValuePair, "=")
		if len(components) != 2 {
			return featureGateMap, fmt.Errorf("expected key=value pairs but %s has %d components", keyValuePair, len(components))
		}
		boolValue, err := strconv.ParseBool(components[1])
		if err != nil {
			return featureGateMap, fmt.Errorf("expected bool value in %s: %v", keyValuePair, err)
		}
		featureGateMap[components[0]] = boolValue
	}
	return featureGateMap, nil
}
