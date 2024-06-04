package eksapi

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
)

func generateUserData(format string, cluster *Cluster) (string, bool, error) {
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
		return "", false, fmt.Errorf("uknown user data format: '%s'", format)
	}
	buf := bytes.Buffer{}
	if err := t.Execute(&buf, templates.UserDataTemplateData{
		APIServerEndpoint:    cluster.endpoint,
		CertificateAuthority: cluster.certificateAuthorityData,
		CIDR:                 cluster.cidr,
		Name:                 cluster.name,
	}); err != nil {
		return "", false, err
	}
	return buf.String(), userDataIsMimePart, nil
}
