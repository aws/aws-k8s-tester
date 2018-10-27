##########################################
FROM gcr.io/k8s-testimages/kubekins-e2e:v20181024-be2f242dd-master
LABEL maintainer "leegyuho@amazon.com"
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
# "ginkgo" is used in
# https://github.com/kubernetes-sigs/aws-alb-ingress-controller
# https://github.com/kubernetes-sigs/aws-ebs-csi-driver
#
# "aws-iam-authenticator" is used for "kubectl" commands to EKS cluster
RUN go get -v github.com/onsi/ginkgo/ginkgo \
  && go get -v github.com/onsi/gomega \
  && go get -v -u github.com/kubernetes-sigs/aws-iam-authenticator/cmd/aws-iam-authenticator
##########################################

##########################################
# https://docs.aws.amazon.com/eks/latest/userguide/configure-kubectl.html
RUN curl -o /workspace/aws-bin/kubectl \
  https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-07-26/bin/linux/amd64/kubectl

# For ALB Ingress Controller scalability tests
RUN git clone https://github.com/wg/wrk.git \
  && pushd wrk \
  && make all \
  && cp ./wrk /workspace/aws-bin/wrk \
  && popd \
  && rm -rf ./wrk

# For "kubetest" EKS plugin
COPY /bin/awstester /workspace/aws-bin/
##########################################

##########################################
RUN chmod +x /workspace/aws-bin/*
##########################################

##########################################
RUN echo ${HOME} \
  && echo ${GOPATH} \
  && go version || true && which go \
  && kubectl version --short --client || true && which kubectl \
  && aws --version || true && which aws \
  && docker --version || true && which docker \
  && wrk --version || true && which wrk \
  && awstester version || true && which awstester \
  && kubetest -h || true && which kubetest
##########################################
