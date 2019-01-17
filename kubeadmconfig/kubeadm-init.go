package kubeadmconfig

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// KubeadmInit defines "kubeadm init" configuration.
// https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-init/
type KubeadmInit struct {
	APIServerAdvertiseAddress string `json:"apiserver-advertise-address,omitempty" kubeadm-init:"apiserver-advertise-address"`
	APIServerBindPort         int    `json:"apiserver-bind-port,omitempty" kubeadm-init:"apiserver-bind-port"`
	PodNetworkCIDR            string `json:"pod-network-cidr,omitempty" kubeadm-init:"pod-network-cidr"`
	ServiceCIDR               string `json:"service-cidr,omitempty" kubeadm-init:"service-cidr"`
}

var defaultKubeadmInit = KubeadmInit{
	APIServerAdvertiseAddress: "",
	APIServerBindPort:         6443,

	// 10.244.0.0/16 for flannel
	PodNetworkCIDR: "10.244.0.0/16",
	// 10.96.0.0/12 for default
	ServiceCIDR: "10.96.0.0/12",
}

func newDefaultKubeadmInit() *KubeadmInit {
	copied := defaultKubeadmInit
	return &copied
}

// Script returns the service file setup script.
func (ka *KubeadmInit) Script() (s string, err error) {
	var fs []string
	fs, err = ka.Flags()
	if err != nil {
		return "", err
	}
	return createScriptInit(scriptInit{
		Exec:     "/usr/bin/kubeadm",
		Flags:    strings.Join(fs, " "),
		UserName: "ec2-user",
	})
}

func createScriptInit(si scriptInit) (string, error) {
	tpl := template.Must(template.New("scriptInitTmpl").Parse(scriptInitTmpl))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, si); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type scriptInit struct {
	Exec     string
	Flags    string
	UserName string
}

// make sure to run as root, otherwise "[ERROR IsPrivilegedUser]: user is not running as root".
const scriptInitTmpl = `#!/usr/bin/env bash

sudo kubeadm init {{ .Flags }} 1>>/var/log/kubeadm-init.log 2>&1

mkdir -p /home/ec2-user/.kube
sudo cp -i /etc/kubernetes/admin.conf /home/ec2-user/.kube/config
sudo chown $(id -u):$(id -g) /home/ec2-user/.kube/config
sudo chown {{ .UserName }}:{{ .UserName }} /home/ec2-user/.kube/config
find /home/ec2-user/.kube/ 1>>/var/log/kubeadm-init.log 2>&1
`

// Flags returns the list of "kubeadm init" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
func (ka *KubeadmInit) Flags() (flags []string, err error) {
	tp, vv := reflect.TypeOf(ka).Elem(), reflect.ValueOf(ka).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kubeadm-init")
		if k == "" {
			continue
		}
		allowZeroValue := tp.Field(i).Tag.Get("allow-zero-value") == "true"
		fieldName := tp.Field(i).Name
		if !strings.HasPrefix(fieldName, "Init") {
			continue
		}

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
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return flags, nil
}

func (ka *KubeadmInit) updateFromEnvs(pfx string) error {
	cc := *ka
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
	*ka = cc
	return nil
}
