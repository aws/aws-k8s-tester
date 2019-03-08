// Package plugins implements kubeadm plugins.
package plugins

import (
	"bytes"
	"text/template"
)

// CreateInstall creates kubeadm install script.
func CreateInstall(ver string) (string, error) {
	tpl := template.Must(template.New("installKubeadmAmazonLinux2Template").Parse(installKubeadmAmazonLinux2Template))
	buf := bytes.NewBuffer(nil)
	kv := kubeadmInfo{Version: ver}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type kubeadmInfo struct {
	Version string
}

// https://kubernetes.io/docs/setup/independent/install-kubeadm/
const installKubeadmAmazonLinux2Template = `

################################## install kubeadm on Amazon Linux 2

cat <<EOF > /tmp/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
#baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-aarch64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
exclude=kube*
EOF
sudo mv /tmp/kubernetes.repo /etc/yum.repos.d/kubernetes.repo

sudo yum install --disableexcludes=kubernetes -y \
  kubelet-{{ .Version }} \
  kubeadm-{{ .Version }} \
  kubectl-{{ .Version }} \
  iproute-tc


kubelet --version
kubeadm version
kubectl version --client=true

sudo iptables --list
sudo iptables -P FORWARD ACCEPT
sudo iptables --list

sudo echo '{ "bip": "192.168.255.1/24" }' > /etc/docker/daemon.json

sudo systemctl daemon-reload
sudo systemctl cat kubelet.service

sudo systemctl enable docker
sudo systemctl start docker

##################################

`
