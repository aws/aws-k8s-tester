FROM public.ecr.aws/amazonlinux/amazonlinux:2023 AS builder
ARG TARGETOS
ARG TARGETARCH
RUN dnf install -y git tar gzip make unzip gcc rsync wget jq
ARG GO_MINOR_VERSION=1.25
RUN curl https://go.dev/dl/?mode=json | jq -r .[].version | grep "^go${GO_MINOR_VERSION}" | head -n1 > go-version.txt
RUN  wget -O go.tar.gz https://go.dev/dl/$(cat go-version.txt).${TARGETOS}-${TARGETARCH}.tar.gz && \
    rm -rf /usr/local/go && \
    tar -C /usr/local -xzf go.tar.gz
ENV GOPATH=/usr/local/go
ENV PATH=$PATH:$GOPATH/bin
ENV GOPROXY=direct

WORKDIR $GOPATH/src/github.com/aws/aws-k8s-tester
COPY . .
RUN go install ./...
RUN go test -c -tags=e2e ./test/... -o $GOPATH/bin/

RUN go install sigs.k8s.io/kubetest2 && \
    go install sigs.k8s.io/kubetest2/kubetest2-tester-exec && \
    go install sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo

FROM public.ecr.aws/amazonlinux/amazonlinux:2023
ARG TARGETOS
ARG TARGETARCH
WORKDIR /workdir
RUN dnf install -y tar gzip unzip wget openssh diffutils
RUN wget -O awscli.zip https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip && \
    unzip awscli.zip && \
    ./aws/install
# we need gsutil from the gcloud CLI for kubetest-tester-ginkgo
RUN dnf install -y python3.13
ARG GCLOUD_SDK_URL=https://dl.google.com/dl/cloudsdk/channels/rapid/google-cloud-sdk.tar.gz
RUN wget -O google-cloud-sdk.tar.gz -q $GCLOUD_SDK_URL && \
    tar xzf google-cloud-sdk.tar.gz -C / && \
    rm google-cloud-sdk.tar.gz && \
    /google-cloud-sdk/install.sh \
        --disable-installation-options \
        --bash-completion=false \
        --path-update=false \
        --usage-reporting=false
ENV PATH=$PATH:/google-cloud-sdk/bin
ARG EKSCTL_VERSION=latest
RUN wget -O eksctl.tar.gz "https://github.com/eksctl-io/eksctl/releases/${EKSCTL_VERSION}/download/eksctl_Linux_${TARGETARCH}.tar.gz" && \
    tar xzf eksctl.tar.gz -C /bin/ && \
    rm eksctl.tar.gz
ARG KUBERNETES_MINOR_VERSION
COPY hack/download-kubernetes-binaries.sh .
RUN ./download-kubernetes-binaries.sh "${KUBERNETES_MINOR_VERSION}" "${TARGETOS}" "${TARGETARCH}"
RUN mkdir /info
ENV PATH=$PATH:/info
RUN cp kubernetes-version.txt /info/
RUN mv kubernetes/*/bin/* /bin/
RUN rm -rf /workdir
COPY --from=builder /usr/local/go/bin/* /bin/
