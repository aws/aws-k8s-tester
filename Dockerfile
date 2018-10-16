##########################################
# TODO: just use kubeins-e2e
# "gcr.io/k8s-testimages/kubekins-e2e" already comes with docker
FROM ubuntu:18.04
LABEL maintainer "leegyuho@amazon.com"

RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

RUN apt-get -y update \
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
  python3 \
  python3-pip \
  python3-setuptools \
  rsync \
  unzip \
  wget \
  xz-utils \
  zip \
  zlib1g-dev \
  lsb-release \
  && apt-get clean \
  && pip3 install awscli --no-cache-dir --upgrade \
  && which aws && aws --version \
  && apt-get -y install \
  python \
  python-dev \
  python-openssl \
  python-pip \
  && pip install --upgrade pip setuptools wheel
##########################################

##########################################
WORKDIR /workspace
ENV TERM xterm
ENV WORKSPACE /workspace
RUN mkdir -p /workspace
ENV PATH /workspace/aws-bin:${PATH}
ENV HOME /workspace
RUN mkdir -p /workspace/aws-bin/ && mkdir -p ${HOME}/.aws/
##########################################

##########################################
ENV GOROOT /usr/local/go
ENV GOPATH /go
ENV PATH ${GOPATH}/bin:${GOROOT}/bin:${PATH}
ENV GO_VERSION 1.11.1
ENV GO_DOWNLOAD_URL https://storage.googleapis.com/golang
RUN rm -rf ${GOROOT} \
  && curl -s ${GO_DOWNLOAD_URL}/go${GO_VERSION}.linux-amd64.tar.gz | tar -v -C /usr/local/ -xz \
  && mkdir -p ${GOPATH}/src ${GOPATH}/bin \
  && go version

RUN go get -v github.com/onsi/ginkgo/ginkgo \
  && go get -v github.com/onsi/gomega

RUN go get -v -u github.com/kubernetes-sigs/aws-iam-authenticator/cmd/aws-iam-authenticator
##########################################

##########################################
# Docker in Docker
# install docker to launch sibling container
# https://github.com/kubernetes/test-infra/bslob/master/images/bootstrap/Dockerfile
# https://github.com/kubernetes/test-infra/blob/master/images/kubekins-e2e/Dockerfile

RUN curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg \
  | apt-key add - && \
  add-apt-repository \
  "deb [arch=amd64] https://download.docker.com/linux/$(. /etc/os-release; echo "$ID") \
  $(lsb_release -cs) stable"

RUN apt-get update && \
  apt-get install -y --no-install-recommends docker-ce=18.06.0* && \
  sed -i 's/cgroupfs_mount$/#cgroupfs_mount\n/' /etc/init.d/docker

# Move Docker's storage location
RUN echo 'DOCKER_OPTS="${DOCKER_OPTS} --data-root=/docker-graph"' | \
  tee --append /etc/default/docker
# NOTE this should be mounted and persisted as a volume ideally (!)
# We will make a fallback one now just in case

RUN mkdir /docker-graph
##########################################

##########################################
RUN git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && cp ./wrk /workspace/aws-bin/wrk \
  && popd \
  && rm -rf ./wrk

# https://docs.aws.amazon.com/eks/latest/userguide/configure-kubectl.html
RUN curl -o /workspace/aws-bin/kubectl \
  https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/kubectl

COPY /bin/awstester /workspace/aws-bin/
RUN chmod +x /workspace/aws-bin/*
##########################################

##########################################
RUN kubectl version --short --client || true && which kubectl \
  && aws --version || true && which aws \
  && docker --version || true && which docker \
  && wrk --version || true && which wrk \
  && awstester -h || true && which awstester
##########################################

##########################################
# add source code and vendor to run e2e tests implemented in "awstester"
# remove this once tests are contributed back to upstream
RUN mkdir -p ${GOPATH}/src/github.com/aws/awstester
ADD . ${GOPATH}/src/github.com/aws/awstester

# clean up locally built binaries
RUN rm -f ${GOPATH}/src/github.com/aws/awstester/awstester
##########################################

##########################################
WORKDIR ${GOPATH}/src/github.com/aws/awstester
##########################################
