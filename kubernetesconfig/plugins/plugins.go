// Package plugins implements Kubernetes plugins.
package plugins

import (
	"bytes"
	"text/template"
)

// CreateInstall creates Kubernetes install script.
func CreateInstall(ver string) (string, error) {
	tpl := template.Must(template.New("installKubernetesAmazonLinux2Template").Parse(installKubernetesAmazonLinux2Template))
	buf := bytes.NewBuffer(nil)
	kv := kubernetesInfo{Version: ver}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type kubernetesInfo struct {
	Version string
}

// https://kubernetes.io/docs/setup/independent/install-kubeadm/
const installKubernetesAmazonLinux2Template = `

################################## install Kubernetes on Amazon Linux 2

cat <<EOF > /tmp/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
exclude=kube*
EOF
sudo cp /tmp/kubernetes.repo /etc/yum.repos.d/kubernetes.repo

cat <<EOF > /tmp/k8s.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
sudo cp /tmp/k8s.conf /etc/sysctl.d/k8s.conf
sudo sysctl --system
sudo sysctl net.bridge.bridge-nf-call-iptables=1

# Set SELinux in permissive mode (effectively disabling it)
setenforce 0
sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config

sudo yum install -y cri-tools ebtables kubernetes-cni socat iproute-tc

RELEASE=v{{ .Version }}

cd /usr/bin
sudo rm -f /usr/bin/{kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}

sudo curl -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/v1.13.0/bin/linux/amd64/{kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}
sudo chmod +x {kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}

curl -sSL "https://raw.githubusercontent.com/kubernetes/kubernetes/${RELEASE}/build/debs/kubelet.service" > /tmp/kubelet.service
cat /tmp/kubelet.service

sudo mkdir -p /etc/systemd/system/kubelet.service.d
sudo cp /tmp/kubelet.service /etc/systemd/system/kubelet.service

sudo systemctl daemon-reload
sudo systemctl cat kubelet.service

kubelet --version
kubectl version --client
crictl --version

##################################

`

/*
https://github.com/kubernetes/kubernetes/blob/master/build/debs/kubelet.service

sudo systemctl enable kubelet && sudo systemctl restart kubelet
sudo systemctl status kubelet --full --no-pager || true
sudo journalctl --no-pager --output=cat -u kubelet
*/
