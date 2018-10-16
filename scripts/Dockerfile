##########################################
# "gcr.io/k8s-testimages/kubekins-e2e" already comes with docker
# FROM gcr.io/k8s-testimages/kubekins-e2e:v20181005-fd9cfb8b0-master
FROM gcr.io/k8s-testimages/kubekins-e2e:latest-master
LABEL maintainer "leegyuho@amazon.com"

RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

RUN go get -v github.com/onsi/ginkgo/ginkgo \
  && go get -v github.com/onsi/gomega \
  && go get -v -u github.com/kubernetes-sigs/aws-iam-authenticator/cmd/aws-iam-authenticator

WORKDIR /workspace
ENV TERM xterm
ENV WORKSPACE /workspace
RUN mkdir -p /workspace
ENV PATH /workspace/aws-bin:${PATH}
ENV HOME /workspace
RUN mkdir -p /workspace/aws-bin/ && mkdir -p ${HOME}/.aws/

RUN git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && cp ./wrk /workspace/aws-bin/wrk \
  && popd \
  && rm -rf ./wrk

# https://docs.aws.amazon.com/eks/latest/userguide/configure-kubectl.html
RUN curl -o /workspace/aws-bin/kubectl \
  https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/kubectl

# RUN curl -o /workspace/aws-bin/awstester \
# https://s3-us-west-2.amazonaws.com/awstester-s3/awstester

COPY /bin/awstester /workspace/aws-bin/

RUN chmod +x /workspace/aws-bin/*

RUN kubectl version --short --client || true && which kubectl \
  && aws --version || true && which aws \
  && docker --version || true && which docker \
  && wrk --version || true && which wrk \
  && awstester -h || true && which awstester
