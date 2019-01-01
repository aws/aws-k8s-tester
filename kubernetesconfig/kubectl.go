package kubernetesconfig

import "fmt"

// Kubectl represents "kubectl" configurations.
type Kubectl struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`
}

var defaultKubectl = Kubectl{
	Path:           "/usr/bin/kubectl",
	DownloadURL:    fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/v%s/bin/linux/amd64/kubectl", defaultKubernetesVersion),
	VersionCommand: "/usr/bin/kubectl version --client",
}

func newDefaultKubectl() *Kubectl {
	copied := defaultKubectl
	return &copied
}
