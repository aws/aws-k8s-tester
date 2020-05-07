package k8sobject

import (
	"regexp"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type NodeInfo struct {
	v1.NodeSystemInfo
	KubeletMinorVersionValue   float64 `json:"kubelet-minor-version-value"`
	KubeProxyMinorVersionValue float64 `json:"kube-proxy-minor-version-value"`
}

// only numbers and dots
var regexVer = regexp.MustCompile("[^.0-9]+")

// ParseVersion parses the version string.
// Returns 0.0 if not valid.
func ParseVersion(s string) (v float64) {
	vs := strings.Split(regexVer.ReplaceAllString(s, ""), ".")
	if len(vs) > 2 {
		v, _ = strconv.ParseFloat(strings.Join(vs[:2], "."), 64)
	}
	return v
}

// ParseNodeInfo parses the node info.
// e.g. {"machineID":"","systemUUID":"","bootID":"","kernelVersion":"4.14.173-137.229.amzn2.x86_64","osImage":"Amazon Linux 2","containerRuntimeVersion":"docker://19.3.6","kubeletVersion":"v1.16.8-eks-e16311","kubeProxyVersion":"v1.16.8-eks-e16311","operatingSystem":"linux","architecture":"amd64"}
func ParseNodeInfo(info v1.NodeSystemInfo) (nodeInfo NodeInfo) {
	nodeInfo = NodeInfo{NodeSystemInfo: info}
	nodeInfo.KubeletMinorVersionValue = ParseVersion(nodeInfo.KubeletVersion)
	nodeInfo.KubeProxyMinorVersionValue = ParseVersion(nodeInfo.KubeProxyVersion)
	return nodeInfo
}
