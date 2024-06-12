package templates

import (
	_ "embed"
	"text/template"
)

//go:embed infra.yaml
var Infrastructure string

var (
	//go:embed unmanaged-nodegroup-efa.yaml.template
	UnmanagedNodegroupEFATemplate string
	UnmanagedNodegroupEFA         = template.Must(template.New("unmanagedNodegroupEFA").Parse(UnmanagedNodegroupEFATemplate))

	//go:embed unmanaged-nodegroup.yaml.template
	unmanagedNodegroupTemplate string
	UnmanagedNodegroup         = template.Must(template.New("unmanagedNodegroup").Parse(unmanagedNodegroupTemplate))
)

type UnmanagedNodegroupTemplateData struct {
	InstanceTypes []string
	Features      map[string]bool
}

type UnmanagedNodegroupEFATemplateData struct {
	Features map[string]bool
}

var (
	//go:embed userdata_bootstrap.sh.mimepart.template
	userDataBootstrapShTemplate string
	UserDataBootstrapSh         = template.Must(template.New("userDataBootstrapSh").Parse(userDataBootstrapShTemplate))

	//go:embed userdata_nodeadm.yaml.mimepart.template
	userDataNodeadmTemplate string
	UserDataNodeadm         = template.Must(template.New("userDataNodeadm").Parse(userDataNodeadmTemplate))

	//go:embed userdata_bottlerocket.toml.template
	userDataBottlerocketTemplate string
	UserDataBottlerocket         = template.Must(template.New("userDataBottlerocket").Parse(userDataBottlerocketTemplate))
)

type UserDataTemplateData struct {
	Name                 string
	CertificateAuthority string
	CIDR                 string
	APIServerEndpoint    string
}

var (
	//go:embed auth_map_role.yaml.template
	authMapRoleTemplate string
	AuthMapRole         = template.Must(template.New("authMapRole").Parse(authMapRoleTemplate))
)

type AuthMapRoleTemplateData struct {
	NodeNameStrategy string
	Rolearn          string
}
