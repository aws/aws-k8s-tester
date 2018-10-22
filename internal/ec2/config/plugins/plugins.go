package plugins

import "fmt"

// headerBash is the bash script header.
const headerBash = `#!/usr/bin/env bash`

// Get returns the plugin.
func Get(ps ...string) (ss string, err error) {
	ss = headerBash
	for _, plugin := range ps {
		switch plugin {
		case "update-ubuntu":
			ss += updateUbuntu
			continue
		case "go1.11.1-ubuntu":
			ss += go1111Ubuntu
			continue
		case "wrk":
			ss += wrk
			continue
		}
		return "", fmt.Errorf("plugin %q not found", plugin)
	}
	return ss, nil
}

const updateUbuntu = `
export HOME=/home/ubuntu
export GOPATH=/home/ubuntu/go

apt-get -y update \
  && apt-get -y install \
  build-essential \
  gcc \
  jq \
  file \
  apt-utils \
  pkg-config \
  software-properties-common \
  apt-transport-https \
  ca-certificates \
  libssl-dev \
  gnupg2 \
  sudo \
  bash \
  curl \
  wget \
  tar \
  git \
  mercurial \
  openssh-client \
  rsync \
  unzip \
  wget \
  xz-utils \
  zip \
  zlib1g-dev \
  lsb-release \
  python3 \
  python3-pip \
  python3-setuptools \
  && apt-get clean \
  && pip3 install awscli --no-cache-dir --upgrade \
  && which aws && aws --version \
  && apt-get -y install \
  python \
  python-dev \
  python-openssl \
  python-pip \
  && pip install --upgrade pip setuptools wheel
`

const go1111Ubuntu = `
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
sudo echo PATH=\"${PATH}\" >> /etc/environment
sudo echo GOPATH=/home/ubuntu/go >> /etc/environment

go version
`

const wrk = `
cd ${HOME} \
  && git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && sudo cp ./wrk /usr/local/bin/wrk \
  && popd \
  && rm -rf ./wrk \
  && wrk --version || true && which wrk

`
