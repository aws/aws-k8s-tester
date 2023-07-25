ARG GO_VERSION
ARG AL_VERSION

FROM golang:${GO_VERSION} AS aws-k8s-tester-builder
ARG GOPROXY
ARG RELEASE_VERSION
RUN go version
ADD ./ /go/src/github.com/aws/aws-k8s-tester
WORKDIR /go/src/github.com/aws/aws-k8s-tester
RUN GOPROXY=${GOPROXY} RELEASE_VERSION=${RELEASE_VERSION} ./hack/build.sh

FROM golang:${GO_VERSION} AS clusterloader2-builder
ARG GOPROXY
ARG OS_TARGET
ARG OS_ARCH
RUN go version
RUN git clone https://github.com/kubernetes/perf-tests.git /perf-tests
WORKDIR /perf-tests/clusterloader2
RUN GOPROXY=${GOPROXY} GOOS=${OS_TARGET} GOARCH=${OS_ARCH} go mod tidy && go build -o ./clusterloader2 ./cmd

FROM amazonlinux:${AL_VERSION}
ARG K8S_VERSION
ARG RELEASE_VERSION
ARG OS_TARGET
ARG OS_ARCH
RUN yum update -y && yum install -y which python3 python3-pip && yum clean all && pip3 install awscli --upgrade --user
ENV PATH=/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN echo ${PATH}
RUN aws --version

COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/aws-k8s-tester-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /aws-k8s-tester
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/ec2-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /ec2-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/ecr-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /ecr-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/eks-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /eks-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/etcd-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /etcd-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/cw-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /cw-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/s3-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /s3-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/sts-utils-${RELEASE_VERSION}-${OS_TARGET}-${OS_ARCH} /sts-utils
# must copy all files from https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/load
# the main config.yaml reads other resource spec (e.g. job.yaml) from the same directory
# RUN curl -o /clusterloader2-test-config.yaml -LO https://raw.githubusercontent.com/kubernetes/perf-tests/master/clusterloader2/testing/load/config.yaml
COPY --from=clusterloader2-builder /perf-tests/clusterloader2/testing/load /clusterloader2-testing-load
COPY --from=clusterloader2-builder /perf-tests/clusterloader2/testing/load/config.yaml /clusterloader2-test-config.yaml
COPY --from=clusterloader2-builder /perf-tests/clusterloader2/clusterloader2 /clusterloader2
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/eks /eks
RUN rm -rf /go/src/github.com/aws/aws-k8s-tester
RUN chmod +x /aws-k8s-tester /cw-utils /ec2-utils /eks-utils /etcd-utils /s3-utils /sts-utils /clusterloader2
WORKDIR /

RUN curl -o /kubectl -LO https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${OS_TARGET}/${OS_ARCH}/kubectl && chmod +x /kubectl && cp /kubectl /usr/local/bin/kubectl
RUN ls /
RUN ls /*.yaml
RUN aws --version
RUN /aws-k8s-tester version
RUN /cw-utils version
RUN /ec2-utils version
RUN /ecr-utils version
RUN /eks-utils version
RUN /etcd-utils version
RUN /s3-utils version
RUN /sts-utils version
RUN cat /clusterloader2-test-config.yaml
RUN /clusterloader2 --help || true
RUN kubectl version --client=true
