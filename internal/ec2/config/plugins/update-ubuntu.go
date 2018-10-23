package plugins

const update_ubuntu = `
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
