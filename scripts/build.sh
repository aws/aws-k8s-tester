#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/build.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [[ -z "${GIT_COMMIT}" ]]; then
  GIT_COMMIT=$(git rev-parse --short=12 HEAD || echo "GitNotFound")
fi

if [[ -z "${RELEASE_VERSION}" ]]; then
  RELEASE_VERSION=v$(date -u '+%Y%m%d.%H%M%S')
fi

if [[ -z "${BUILD_TIME}" ]]; then
  BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
fi

echo "GIT_COMMIT:" ${GIT_COMMIT}
echo "RELEASE_VERSION:" ${RELEASE_VERSION}
echo "BUILD_TIME:" ${BUILD_TIME}

mkdir -p ./bin

_BUILD_TARGETS=${BUILD_TARGETS:-'linux darwin'}

for os in ${_BUILD_TARGETS}; do
  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/aws-k8s-tester-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/aws-k8s-tester

  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/aws-utils-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/aws-utils

  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/ec2-utils-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/ec2-utils

  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/eks-utils-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/eks-utils

  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/etcd-utils-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/etcd-utils

  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -mod=vendor -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/cw-utils-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/cw-utils
done

if [[ "${OSTYPE}" == "linux-gnu" ]]; then
  ./bin/aws-k8s-tester-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  ./bin/aws-utils-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  ./bin/ec2-utils-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  ./bin/eks-utils-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  ./bin/etcd-utils-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  ./bin/cw-utils-${RELEASE_VERSION}-linux-$(go env GOARCH) version
  cp ./bin/aws-k8s-tester-${RELEASE_VERSION}-linux-$(go env GOARCH) ./aws-k8s-tester
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  ./bin/aws-k8s-tester-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
  ./bin/aws-utils-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
  ./bin/ec2-utils-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
  ./bin/eks-utils-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
  ./bin/etcd-utils-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
  ./bin/cw-utils-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
fi

echo "Success!"
find ./bin
