FROM golang AS aws-k8s-tester-builder
RUN go version
ADD ./ /go/src/github.com/aws/aws-k8s-tester
WORKDIR /go/src/github.com/aws/aws-k8s-tester
ARG RELEASE_VERSION=latest
ARG BUILD_TARGETS=linux
RUN ./scripts/build.sh

FROM amazonlinux:latest
RUN yum update -y && yum install -y which python3 pip3 && yum clean all && pip3 install awscli --upgrade --user
ENV PATH=/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN echo ${PATH}
RUN /root/.local/bin/aws --version
RUN aws --version

ARG RELEASE_VERSION=latest
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/aws-k8s-tester-${RELEASE_VERSION}-linux-amd64 /aws-k8s-tester
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/eks-utils-${RELEASE_VERSION}-linux-amd64 /eks-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/etcd-utils-${RELEASE_VERSION}-linux-amd64 /etcd-utils
WORKDIR /
RUN rm -rf /go/src/github.com/aws/aws-k8s-tester

RUN /aws-k8s-tester version
