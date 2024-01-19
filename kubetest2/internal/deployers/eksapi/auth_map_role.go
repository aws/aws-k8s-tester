package eksapi

import (
	"bytes"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/deployers/eksapi/templates"
)

func generateAuthMapRole(nodeNameStrategy string, rolearn string) (string, error) {
	template := templates.AuthMapRole
	buf := bytes.Buffer{}
	if err := template.Execute(&buf, templates.AuthMapRoleTemplateData{
		NodeNameStrategy:  nodeNameStrategy,
		Rolearn:           rolearn,
	}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
