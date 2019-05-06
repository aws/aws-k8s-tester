// Package plugins implements kubeadm plugins.
package plugins

import (
	"bytes"
	"text/template"
)

// CreateInstallAL2 creates kubeadm install script.
func CreateInstallAL2(ver string) (string, error) {
	tpl := template.Must(template.New("installKubeadmAL2").Parse(installKubeadmAL2))
	buf := bytes.NewBuffer(nil)
	kv := kubeadmInfo{Version: ver}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// CreateInstallUbuntu creates kubeadm install script.
func CreateInstallUbuntu(ver string) (string, error) {
	tpl := template.Must(template.New("installKubeadmUbuntu").Parse(installKubeadmUbuntu))
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
const installKubeadmAL2 = `

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

sudo yum install \
  --disableexcludes=kubernetes -y \
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

// https://kubernetes.io/docs/setup/independent/install-kubeadm/
// https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/
const installKubeadmUbuntu = `

################################## install kubeadm on Ubuntu

sudo apt-get update -y
sudo apt-get install -y apt-transport-https curl

curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -

sudo mkdir -p /etc/apt/sources.list.d/
sudo rm -f /etc/apt/sources.list.d/kubernetes.list

# environment variable for correct distribution
export UBUNTU_RELEASE="kubernetes-$(lsb_release -c -s)"
echo "deb https://apt.kubernetes.io/ $UBUNTU_RELEASE main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list
sudo cat /etc/apt/sources.list.d/kubernetes.list

sudo apt-get update -y
sudo apt-get install -y \
  kubelet={{ .Version }} \
  kubeadm={{ .Version }} \
  kubectl={{ .Version }}
sudo apt-mark hold kubelet kubeadm kubectl

kubelet --version
kubeadm version
kubectl version --client=true

sudo iptables --list
sudo systemctl daemon-reload
sudo systemctl cat kubelet.service

##################################

`
