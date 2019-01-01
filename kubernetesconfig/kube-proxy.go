package kubernetesconfig

import (
	"fmt"
	"reflect"
)

// KubeProxy defines "kube-proxy" configuration.
// Reference: https://godoc.org/k8s.io/kube-proxy/config/v1alpha1#KubeProxyConfiguration.
type KubeProxy struct {
	// Image is the container image name and tag for kube-proxy to run as a static pod.
	Image string `json:"image"`

	ClusterCIDR         string `json:"cluster-cidr" kube-proxy:"cluster-cidr"`
	ConntrackMaxPerCore int64  `json:"conntrack-max-per-core" kube-proxy:"conntrack-max-per-core"`
	HostnameOverride    string `json:"hostname-override" kube-proxy:"hostname-override"`
	Kubeconfig          string `json:"kubeconfig" kube-proxy:"kubeconfig"`
	Master              string `json:"master" kube-proxy:"master"`
	OOMScoreAdj         int    `json:"oom-score-adj" kube-proxy:"oom-score-adj"`
	ResourceContainer   string `json:"resource-container" kube-proxy:"resource-container" allow-zero-value:"true"`
	V                   int    `json:"v" kube-proxy:"v"`
}

var defaultKubeProxyMasterNodes = KubeProxy{
	Image: fmt.Sprintf("k8s.gcr.io/kube-proxy:v%s", defaultKubernetesVersion),

	ClusterCIDR:         "100.96.0.0/11",
	ConntrackMaxPerCore: 131072,
	HostnameOverride:    "PRIVATE_DNS",
	Kubeconfig:          "/var/lib/kube-proxy/kubeconfig",
	Master:              "https://127.0.0.1",
	OOMScoreAdj:         -998,
	ResourceContainer:   "",
}

func newDefaultKubeProxyMasterNodes() *KubeProxy {
	copied := defaultKubeProxyMasterNodes
	return &copied
}

var defaultKubeProxyWorkerNodes = KubeProxy{
	Image: fmt.Sprintf("k8s.gcr.io/kube-proxy:v%s", defaultKubernetesVersion),

	ClusterCIDR:         "100.96.0.0/11",
	ConntrackMaxPerCore: 131072,
	HostnameOverride:    "PRIVATE_DNS",
	Kubeconfig:          "/var/lib/kube-proxy/kubeconfig",
	Master:              "https://api.internal.leegyuho-aws-k8s-tester.k8s.local",
	OOMScoreAdj:         -998,
	ResourceContainer:   "",
	V:                   2,
}

func newDefaultKubeProxyWorkerNodes() *KubeProxy {
	copied := defaultKubeProxyWorkerNodes
	return &copied
}

// Flags returns the list of "kube-proxy" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kb *KubeProxy) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kb).Elem(), reflect.ValueOf(kb).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kube-proxy")
		if k == "" {
			continue
		}
		allowZeroValue := tp.Field(i).Tag.Get("allow-zero-value") == "true"

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			} else if allowZeroValue {
				// e.g. handle --resource-container=""
				flags = append(flags, fmt.Sprintf(`--%s=""`, k))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=0`, k))
			}

		case reflect.Bool:
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}
