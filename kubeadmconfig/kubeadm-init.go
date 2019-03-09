package kubeadmconfig

import (
	"bytes"
	"text/template"
)

// KubeadmInit defines "kubeadm init" configuration.
// https://kubernetes.io/docs/reference/setup-tools/kubeadm/kubeadm-init/
type KubeadmInit struct {
	MasterNodePrivateDNS string `json:"master-node-private-dns"`
}

// Script returns the service file setup script.
func (ka *KubeadmInit) Script() (s string, err error) {
	tpl := template.Must(template.New("scriptInitTmpl").Parse(scriptInitTmpl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, ka); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// make sure to run as root, otherwise "[ERROR IsPrivilegedUser]: user is not running as root".
const scriptInitTmpl = `#!/usr/bin/env bash

mkdir -p /home/ec2-user/.kube

cat > /home/ec2-user/kubeadm.init.yaml <<EOF
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
nodeRegistration:
  name: {{ .MasterNodePrivateDNS }}
  kubeletExtraArgs:
    cloud-provider: aws


---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration

apiServer:
  extraArgs:
    cloud-provider: aws

controllerManager:
  extraArgs:
    cloud-provider: aws

EOF
cat /home/ec2-user/kubeadm.init.yaml


sudo kubeadm init --config /home/ec2-user/kubeadm.init.yaml 1>>/var/log/kubeadm.init.log 2>&1

`
