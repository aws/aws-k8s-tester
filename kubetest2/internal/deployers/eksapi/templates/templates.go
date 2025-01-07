package templates

import (
	_ "embed"
	"text/template"
)

//go:embed infra.yaml
var Infrastructure string

//go:embed unmanaged-nodegroup-efa.yaml
var UnmanagedNodegroupEFA string

var (
	//go:embed unmanaged-nodegroup.yaml.template
	unmanagedNodegroupTemplate string
	UnmanagedNodegroup         = template.Must(template.New("unmanagedNodegroup").Parse(unmanagedNodegroupTemplate))
)

type UnmanagedNodegroupTemplateData struct {
	KubernetesVersion string
	InstanceTypes     []string
}

type BusyboxDeploymentTemplateData struct {
	Nodes int
}

type NvidiaStaticClusterNodepoolTemplateData struct {
	Arch          string
	InstanceTypes []string
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

	//go:embed busybox_deployment.yaml.template
	busyboxDeploymentTemplate string
	BusyboxDeployment         = template.Must(template.New("busyboxDeployment").Parse(busyboxDeploymentTemplate))

	//go:embed nvidia_static_cluster_nodepool.yaml.template
	nvidiaStaticClusterNodepoolTemplate string
	NvidiaStaticClusterNodepool         = template.Must(template.New("nvidiaStaticClusterNodepool").Parse(nvidiaStaticClusterNodepoolTemplate))
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
