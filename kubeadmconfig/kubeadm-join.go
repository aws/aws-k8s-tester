package kubeadmconfig

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"text/template"
)

// KubeadmJoin defines "kubeadm join" configuration.
// https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-join/
type KubeadmJoin struct {
	WorkerNodePrivateDNS string `json:"worker-node-private-dns"`

	RawCommand               string `json:"raw-command"`
	Target                   string `json:"target"`
	Token                    string `json:"token,omitempty" kubeadm-join:"token"`
	DiscoveryTokenCACertHash string `json:"discovery-token-ca-cert-hash,omitempty" kubeadm-join:"discovery-token-ca-cert-hash"`
	IgnorePreflightErrors    string `json:"ignore-preflight-errors,omitempty" kubeadm-join:"ignore-preflight-errors"`
}

var defaultKubeadmJoin = KubeadmJoin{
	IgnorePreflightErrors: "cri",
}

func newDefaultKubeadmJoin() *KubeadmJoin {
	copied := defaultKubeadmJoin
	return &copied
}

// Flags returns the list of "kubeadm join" flags.
// Make sure to validate the configuration with "ValidateAndSetDefaults".
// Make sure to run as root, otherwise "[ERROR IsPrivilegedUser]: user is not running as root".
func (ka *KubeadmJoin) Flags() (flags []string, err error) {
	arg := ka.Target
	if arg == "" {
		return nil, errors.New("unknown 'kubeadm join' target")
	}
	tp, vv := reflect.TypeOf(ka).Elem(), reflect.ValueOf(ka).Elem()
	for i := 0; i < tp.NumField(); i++ {
		k := tp.Field(i).Tag.Get("kubeadm-join")
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
			flags = append(flags, fmt.Sprintf("--%s=%v", k, vv.Field(i).Bool()))

		default:
			return nil, fmt.Errorf("unknown %q", k)
		}
	}
	return append([]string{arg}, flags...), nil
}

// Command returns the "kubectl join" command.
func (ka *KubeadmJoin) Command() (cmd string, err error) {
	var joinFlags []string
	joinFlags, err = ka.Flags()
	if err != nil {
		return "", err
	}
	cmd = fmt.Sprintf("sudo kubeadm join %s", strings.Join(joinFlags, " "))
	return cmd, nil
}

// Script returns the service file setup script.
func (ka *KubeadmJoin) Script() (s string, err error) {
	tpl := template.Must(template.New("scriptJoinTmpl").Parse(scriptJoinTmpl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, ka); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// make sure to run as root, otherwise "[ERROR IsPrivilegedUser]: user is not running as root".
const scriptJoinTmpl = `#!/usr/bin/env bash

mkdir -p /home/ec2-user/.kube

cat > /home/ec2-user/kubeadm.join.yaml <<EOF
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
nodeRegistration:
  name: {{ .WorkerNodePrivateDNS }}
  kubeletExtraArgs:
    cloud-provider: aws

discovery:
  bootstrapToken:
    apiServerEndpoint: {{ .Target }}
    token: {{ .Token }}
    caCertHashes:
    - {{ .DiscoveryTokenCACertHash }}

EOF
cat /home/ec2-user/kubeadm.join.yaml

sudo kubeadm join --config /home/ec2-user/kubeadm.join.yaml 1>>/var/log/kubeadm.join.log 2>&1

`
