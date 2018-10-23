package plugins

const go_1_11_1_ubuntu = `
export HOME=/home/ubuntu
export GOPATH=/home/ubuntu/go

GO_VERSION=1.11.1
GOOGLE_URL=https://storage.googleapis.com/golang
DOWNLOAD_URL=${GOOGLE_URL}

sudo curl -s ${DOWNLOAD_URL}/go$GO_VERSION.linux-amd64.tar.gz | sudo tar -v -C /usr/local/ -xz

mkdir -p ${GOPATH}/bin/
mkdir -p ${GOPATH}/src/github.com
mkdir -p ${GOPATH}/src/k8s.io
mkdir -p ${GOPATH}/src/sigs.k8s.io

if grep -q GOPATH "${HOME}/.bashrc"; then
  echo "bashrc already has GOPATH";
else
  echo "adding GOPATH to bashrc";
  echo "export GOPATH=${HOME}/go" >> ${HOME}/.bashrc;
  PATH_VAR=$PATH":/usr/local/go/bin:${HOME}/go/bin";
  echo "export PATH=$(echo $PATH_VAR)" >> ${HOME}/.bashrc;
  source ${HOME}/.bashrc;
fi

source ${HOME}/.bashrc
export PATH=$PATH:/usr/local/go/bin:${HOME}/go/bin

sudo echo PATH=${PATH} > /etc/environment
sudo echo GOPATH=/home/ubuntu/go >> /etc/environment

go version
`
