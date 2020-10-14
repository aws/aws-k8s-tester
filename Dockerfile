FROM golang AS aws-k8s-tester-builder
RUN go version
ADD ./ /go/src/github.com/aws/aws-k8s-tester
WORKDIR /go/src/github.com/aws/aws-k8s-tester
ARG RELEASE_VERSION=latest
ARG OS_TARGETS=linux
RUN ./hack/build.sh

FROM amazonlinux:latest
RUN yum update -y && yum install -y which python3 pip3 && yum clean all && pip3 install awscli --upgrade --user
ENV PATH=/root/.local/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN echo ${PATH}
RUN /root/.local/bin/aws --version
RUN aws --version

ARG RELEASE_VERSION=latest
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/aws-k8s-tester-${RELEASE_VERSION}-linux-amd64 /aws-k8s-tester
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/ec2-utils-${RELEASE_VERSION}-linux-amd64 /ec2-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/ecr-utils-${RELEASE_VERSION}-linux-amd64 /ecr-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/eks-utils-${RELEASE_VERSION}-linux-amd64 /eks-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/etcd-utils-${RELEASE_VERSION}-linux-amd64 /etcd-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/cw-utils-${RELEASE_VERSION}-linux-amd64 /cw-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/s3-utils-${RELEASE_VERSION}-linux-amd64 /s3-utils
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/bin/sts-utils-${RELEASE_VERSION}-linux-amd64 /sts-utils
# must copy all files from https://github.com/kubernetes/perf-tests/tree/master/clusterloader2/testing/load
# the main config.yaml reads other resource spec (e.g. job.yaml) from the same directory
# RUN curl -o /clusterloader2-test-config.yaml -LO https://raw.githubusercontent.com/kubernetes/perf-tests/master/clusterloader2/testing/load/config.yaml
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/_tmp/clusterloader2-testing-load/ /
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/_tmp/clusterloader2-testing-load/config.yaml /clusterloader2-test-config.yaml
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/_tmp/clusterloader2 /clusterloader2
COPY --from=aws-k8s-tester-builder /go/src/github.com/aws/aws-k8s-tester/eks /eks
RUN rm -rf /go/src/github.com/aws/aws-k8s-tester
RUN chmod +x /aws-k8s-tester /cw-utils /ec2-utils /eks-utils /etcd-utils /s3-utils /sts-utils /clusterloader2
WORKDIR /

RUN curl -o /kubectl -LO https://storage.googleapis.com/kubernetes-release/release/v1.18.9/bin/linux/amd64/kubectl && chmod +x /kubectl && cp /kubectl /usr/local/bin/kubectl
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
