##########################################
# "gcr.io/k8s-testimages/kubekins-e2e" already comes with docker
# FROM gcr.io/k8s-testimages/kubekins-e2e:v20181005-fd9cfb8b0-master
FROM gcr.io/k8s-testimages/kubekins-e2e:latest-master
LABEL maintainer "leegyuho@amazon.com"
##########################################

##########################################
RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
##########################################

##########################################
RUN go get -v github.com/onsi/ginkgo/ginkgo \
  && go get -v github.com/onsi/gomega \
  && go get -v -u github.com/kubernetes-sigs/aws-iam-authenticator/cmd/aws-iam-authenticator
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
RUN git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && cp ./wrk /workspace/aws-bin/wrk \
  && popd \
  && rm -rf ./wrk
##########################################

##########################################
# remove this once is merged upstream
RUN mkdir -p $GOPATH/src/k8s.io
RUN git clone https://github.com/gyuho/test-infra.git --branch eks-plugin $GOPATH/src/k8s.io/test-infra \
  && pushd $GOPATH/src/k8s.io/test-infra \
  && go build -v -o /workspace/aws-bin/kubetest ./kubetest \
  && popd
##########################################

##########################################
# https://docs.aws.amazon.com/eks/latest/userguide/configure-kubectl.html
RUN curl -o /workspace/aws-bin/kubectl \
  https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/kubectl

# RUN curl -o /workspace/aws-bin/awstester \
# https://s3-us-west-2.amazonaws.com/awstester-s3/awstester
##########################################

##########################################
COPY /bin/awstester /workspace/aws-bin/

RUN chmod +x /workspace/aws-bin/*
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

RUN apt-get -y update \
  && apt-get -y install \
  build-essential \
  gcc \
  apt-utils \
  pkg-config \
  software-properties-common \
  apt-transport-https \
  libssl-dev \
  sudo \
  bash \
  curl \
  wget \
  tar \
  git \
  netcat \
  libaspell-dev \
  libhunspell-dev \
  hunspell-en-us \
  aspell-en \
  shellcheck \
  && apt-get -y update \
  && apt-get -y upgrade \
  && apt-get -y autoremove \
  && apt-get -y autoclean
##########################################

##########################################
RUN mkdir -p ${GOPATH}/src/go.etcd.io/etcd
WORKDIR ${GOPATH}/src/go.etcd.io/etcd
##########################################

##########################################
RUN go get -v -u -tags spell github.com/chzchzchz/goword \
  && go get -v -u github.com/coreos/license-bill-of-materials \
  && go get -v -u github.com/mgechev/revive \
  && go get -v -u github.com/mdempsky/unconvert \
  && go get -v -u mvdan.cc/unparam \
  && go get -v -u honnef.co/go/tools/cmd/gosimple \
  && go get -v -u honnef.co/go/tools/cmd/unused \
  && go get -v -u honnef.co/go/tools/cmd/staticcheck \
  && go get -v -u github.com/gyuho/gocovmerge \
  && go get -v -u github.com/gordonklaus/ineffassign \
  && go get -v -u github.com/alexkohler/nakedret \
  && curl -s https://codecov.io/bash >/codecov \
  && chmod 700 /codecov

RUN echo ${HOME} \
  && echo ${GOPATH} \
  && go version || true && which go \
  && kubectl version --short --client || true && which kubectl \
  && aws --version || true && which aws \
  && docker --version || true && which docker \
  && wrk --version || true && which wrk \
  && awstester -h || true && which awstester \
  && kubetest -h || true && which kubetest
##########################################
