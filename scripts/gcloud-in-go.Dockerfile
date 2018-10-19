##########################################
FROM gcr.io/k8s-testimages/gcloud-in-go:v20180927-6b4facbe6
LABEL maintainer "leegyuho@amazon.com"
##########################################

##########################################
RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections
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
  && go version || true && which go
##########################################
