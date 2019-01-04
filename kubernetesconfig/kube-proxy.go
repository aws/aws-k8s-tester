package kubernetesconfig

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// KubeProxy defines "kube-proxy" configuration.
// Reference: https://godoc.org/k8s.io/kube-proxy/config/v1alpha1#KubeProxyConfiguration.
type KubeProxy struct {
	Path           string `json:"path"`
	DownloadURL    string `json:"download-url"`
	VersionCommand string `json:"version-command"`

	// TODO: support running as a static pod
	// Image is the container image name and tag for kube-proxy to run as a static pod.
	// Image string `json:"image"`

	// UserName is the user name used for running init scripts or SSH access.
	UserName string `json:"user-name,omitempty"`

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
	Path:           "/usr/bin/kube-proxy",
	DownloadURL:    fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/v%s/bin/linux/amd64/kube-proxy", defaultKubernetesVersion),
	VersionCommand: "/usr/bin/kube-proxy --version",

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
	Path:           "/usr/bin/kube-proxy",
	DownloadURL:    fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/v%s/bin/linux/amd64/kube-proxy", defaultKubernetesVersion),
	VersionCommand: "/usr/bin/kube-proxy --version",

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

func (kb *KubeProxy) updateFromEnvs(pfx string) error {
	cc := *kb
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
	*kb = cc
	return nil
}

// Service returns a script to configure kube-proxy systemd service file.
func (kb *KubeProxy) Service() (s string, err error) {
	tpl := template.Must(template.New("kubeProxyTemplate").Parse(kubeProxyTemplate))
	buf := bytes.NewBuffer(nil)
	kv := kubeProxyTemplateInfo{KubeProxyPath: kb.Path}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type kubeProxyTemplateInfo struct {
	KubeProxyPath string
}

const kubeProxyTemplate = `#!/usr/bin/env bash

sudo systemctl stop kube-proxy.service || true

sudo rm -rf /var/lib/kube-proxy/
sudo mkdir -p /var/lib/kube-proxy/
sudo rm -f /var/lib/kube-proxy/kubeconfig

rm -f /tmp/kube-proxy.service
cat <<EOF > /tmp/kube-proxy.service
[Unit]
Description=kube-proxy: The Kubernetes API Server
Documentation=http://kubernetes.io/docs/
After=docker.service

[Service]
EnvironmentFile=/etc/sysconfig/kube-proxy
ExecStart={{ .KubeProxyPath }} "\$KUBE_PROXY_FLAGS"
Restart=always
RestartSec=2s
StartLimitInterval=0
KillMode=process
User=root

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/kube-proxy.service

sudo mkdir -p /etc/systemd/system/kube-proxy.service.d
sudo cp /tmp/kube-proxy.service /etc/systemd/system/kube-proxy.service

sudo systemctl daemon-reload
sudo systemctl cat kube-proxy.service
`

// Sysconfig returns "/etc/sysconfig/kube-proxy" file.
func (kb *KubeProxy) Sysconfig() (s string, err error) {
	var fs []string
	fs, err = kb.Flags()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`KUBE_PROXY_FLAGS="%s"
HOME="/home/%s"
`, strings.Join(fs, " "),
		kb.UserName,
	), nil
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
