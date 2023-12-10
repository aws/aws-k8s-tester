package templates

import (
	_ "embed"
	"text/template"
)

//go:embed infra.yaml
var Infrastructure string

//go:embed unmanaged-nodegroup.yaml.template
var unmanagedNodegroupTemplate string

var UnmanagedNodegroup *template.Template

func init() {
	t, err := template.New("unmanaged-nodegroup").Parse(unmanagedNodegroupTemplate)
	if err != nil {
		panic(err)
	}
	UnmanagedNodegroup = t
}
