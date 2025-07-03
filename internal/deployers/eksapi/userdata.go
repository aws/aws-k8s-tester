package eksapi

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/aws/aws-k8s-tester/internal/deployers/eksapi/templates"
)

func generateUserData(format string, cluster *Cluster, opts *deployerOptions) (string, bool, error) {
	userDataIsMimePart := true
	var t *template.Template
	switch format {
	case "bootstrap.sh":
		t = templates.UserDataBootstrapSh
	case "nodeadm":
		// TODO: replace the YAML template with proper usage of the nodeadm API go types
		t = templates.UserDataNodeadm
	case "bottlerocket":
		t = templates.UserDataBottlerocket
		userDataIsMimePart = false
	default:
		return "", false, fmt.Errorf("unknown user data format: '%s'", format)
	}

	kubeletFeatureGates := map[string]bool{}
	// DRA is in beta for 1.33, and so needs to be explicitly enabled.
	if opts.KubernetesVersion == "1.33" {
		kubeletFeatureGates["DynamicResourceAllocation"] = true
	}

	nodeadmFeatureGateMap := make(map[string]bool)
	// TODO: make this more reusable for kubelet feature gates if exposing those, too
	for _, keyValuePair := range opts.NodeadmFeatureGates {
		components := strings.Split(keyValuePair, "=")
		if len(components) != 2 {
			return "", false, fmt.Errorf("expected key=value pairs but %s has %d components", keyValuePair, len(components))
		}
		boolValue, err := strconv.ParseBool(components[1])
		if err != nil {
			return "", false, fmt.Errorf("expected bool value in %s: %v", keyValuePair, err)
		}
		nodeadmFeatureGateMap[components[0]] = boolValue
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, templates.UserDataTemplateData{
		APIServerEndpoint:    cluster.endpoint,
		CertificateAuthority: cluster.certificateAuthorityData,
		CIDR:                 cluster.cidr,
		Name:                 cluster.name,
		KubeletFeatureGates:  kubeletFeatureGates,
		NodeadmFeatureGates:  nodeadmFeatureGateMap,
	}); err != nil {
		return "", false, err
	}
	return buf.String(), userDataIsMimePart, nil
}
