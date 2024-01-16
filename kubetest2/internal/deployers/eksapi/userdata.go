package eksapi

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
)

func generateUserData(format string, cluster *Cluster) (string, error) {
	var t *template.Template
	switch format {
	case "bootstrap.sh":
		t = templates.UserDataBootstrapSh
	case "nodeadm":
		// TODO: replace the YAML template with proper usage of the nodeadm API go types
		t = templates.UserDataNodeadm
	default:
		return "", fmt.Errorf("uknown user data format: '%s'", format)
	}
	buf := bytes.Buffer{}
	if err := t.Execute(&buf, templates.UserDataTemplateData{
		APIServerEndpoint:    cluster.endpoint,
		CertificateAuthority: cluster.certificateAuthorityData,
		CIDR:                 cluster.cidr,
		Name:                 cluster.name,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
