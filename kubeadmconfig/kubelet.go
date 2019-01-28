package kubeadmconfig

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Kubelet represents "kubelet" configurations.
type Kubelet struct {
	// UserName is the user name used for running init scripts or SSH access.
	UserName string `json:"user-name,omitempty"`

	BootstrapKubeconfig string `json:"bootstrap-kubeconfig,omitempty" kubelet:"bootstrap-kubeconfig"`
	Kubeconfig          string `json:"kubeconfig,omitempty" kubelet:"kubeconfig"`
	PodManifestPath     string `json:"pod-manifest-path,omitempty" kubelet:"pod-manifest-path"`
	AllowPrivileged     bool   `json:"allow-privileged,omitempty" kubelet:"allow-privileged"`
	AnonymousAuth       bool   `json:"anonymous-auth,omitempty" kubelet:"anonymous-auth"`
	CgroupDriver        string `json:"cgroup-driver,omitempty" kubelet:"cgroup-driver"`
	CgroupRoot          string `json:"cgroup-root,omitempty" kubelet:"cgroup-root"`
	RotateCertificates  bool   `json:"rotate-certificates,omitempty" kubelet:"rotate-certificates"`
	ClientCAFile        string `json:"client-ca-file,omitempty" kubelet:"client-ca-file"`
	CloudProvider       string `json:"cloud-provider,omitempty" kubelet:"cloud-provider"`

	// ClusterDNS is a comma-separated list of DNS server IP addresses.
	// See https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#introduction for more detail.
	ClusterDNS string `json:"cluster-dns,omitempty" kubelet:"cluster-dns"`
	// ClusterDomain is by default "cluster.local".
	// See https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#introduction for more detail.
	ClusterDomain string `json:"cluster-domain,omitempty" kubelet:"cluster-domain"`

	AuthorizationMode       string `json:"authorization-mode,omitempty" kubelet:"authorization-mode"`
	EnableDebuggingHandlers bool   `json:"enable-debugging-handlers,omitempty" kubelet:"enable-debugging-handlers"`
	EvictionHard            string `json:"eviction-hard,omitempty" kubelet:"eviction-hard"`
	FeatureGates            string `json:"feature-gates,omitempty" kubelet:"feature-gates"`
	HostnameOverride        string `json:"hostname-override,omitempty" kubelet:"hostname-override"`
	NetworkPluginMTU        int64  `json:"network-plugin-mtu,omitempty" kubelet:"network-plugin-mtu"`
	NetworkPlugin           string `json:"network-plugin,omitempty" kubelet:"network-plugin"`
	NodeLabels              string `json:"node-labels,omitempty" kubelet:"node-labels"`
	NonMasqueradeCIDR       string `json:"non-masquerade-cidr,omitempty" kubelet:"non-masquerade-cidr"`
	PodInfraContainerImage  string `json:"pod-infra-container-image,omitempty" kubelet:"pod-infra-container-image"`
	RegisterSchedulable     bool   `json:"register-schedulable,omitempty" kubelet:"register-schedulable"`
	RegisterWithTaints      string `json:"register-with-taints,omitempty" kubelet:"register-with-taints"`
	V                       int    `json:"v" kubelet:"v"`
	CNIBinDir               string `json:"cni-bin-dir,omitempty" kubelet:"cni-bin-dir"`
	CNIConfDir              string `json:"cni-conf-dir,omitempty" kubelet:"cni-conf-dir"`
}

var defaultKubelet = Kubelet{
	// TODO: use Amazon Linux 2 with "ec2-user"
	UserName:            "ubuntu",

	BootstrapKubeconfig: "/etc/kubernetes/bootstrap-kubelet.conf",
	Kubeconfig:          "/etc/kubernetes/kubelet.conf",
	PodManifestPath:     "/etc/kubernetes/manifests",
	AllowPrivileged:     true,
	ClusterDNS:          "10.96.0.10",
	ClusterDomain:       "cluster.local",
	AuthorizationMode:   "Webhook",

	CgroupDriver: "systemd",

	// ClientCAFile: "/etc/kubernetes/pki/ca.crt",
	RotateCertificates: true,

	V: 2,
}

func newDefaultKubelet() *Kubelet {
	copied := defaultKubelet
	return &copied
}

func (kl *Kubelet) updateFromEnvs(pfx string) error {
	cc := *kl
	tp, vv := reflect.TypeOf(&cc).Elem(), reflect.ValueOf(&cc).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.Replace(jv, "-", "_", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv
		if os.Getenv(env) == "" {
			continue
		}
		sv := os.Getenv(env)

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			vv.Field(i).SetString(sv)

		case reflect.Bool:
			bb, err := strconv.ParseBool(sv)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetBool(bb)

		case reflect.Int, reflect.Int32, reflect.Int64:
			// if tp.Field(i).Name { continue }
			iv, err := strconv.ParseInt(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetInt(iv)

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			iv, err := strconv.ParseUint(sv, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetUint(iv)

		case reflect.Float32, reflect.Float64:
			fv, err := strconv.ParseFloat(sv, 64)
			if err != nil {
				return fmt.Errorf("failed to parse %q (%q, %v)", sv, env, err)
			}
			vv.Field(i).SetFloat(fv)

		case reflect.Slice:
			ss := strings.Split(sv, ",")
			slice := reflect.MakeSlice(reflect.TypeOf([]string{}), len(ss), len(ss))
			for i := range ss {
				slice.Index(i).SetString(ss[i])
			}
			vv.Field(i).Set(slice)

		default:
			return fmt.Errorf("%q (%v) is not supported as an env", env, vv.Field(i).Type())
		}
	}
	*kl = cc
	return nil
}

// Sysconfig returns "/etc/sysconfig/kubelet" file.
func (kl *Kubelet) Sysconfig() (s string, err error) {
	var fs []string
	fs, err = kl.Flags()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`KUBELET_FLAGS="%s"
HOME="/home/%s"
`, strings.Join(fs, " "),
		kl.UserName,
	), nil
}

// Flags returns the list of "kubelet" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (kl *Kubelet) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(kl).Elem(), reflect.ValueOf(kl).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kubelet")
		if k == "" {
			continue
		}
		allowZeroValue := tp.Field(i).Tag.Get("allow-zero-value") == "true"

		switch vv.Field(i).Type().Kind() {
		case reflect.String:
			if vv.Field(i).String() != "" {
				flags = append(flags, fmt.Sprintf("--%s=%s", k, vv.Field(i).String()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=""`, k))
			}

		case reflect.Int, reflect.Int32, reflect.Int64:
			if vv.Field(i).Int() != 0 {
				flags = append(flags, fmt.Sprintf("--%s=%d", k, vv.Field(i).Int()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=0`, k))
			}

		case reflect.Bool:
			if vv.Field(i).Bool() {
				flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))
			} else if allowZeroValue {
				flags = append(flags, fmt.Sprintf(`--%s=false`, k))
			}

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}
