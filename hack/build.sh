#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ hack/build.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short=12 HEAD || echo "GitNotFound")}
RELEASE_VERSION=${RELEASE_VERSION:-v$(date -u '+%Y%m%d.%H%M%S')}
BUILD_TIME=${BUILD_TIME:-$(date -u '+%Y-%m-%d_%H:%M:%S')}
echo "GIT_COMMIT:" ${GIT_COMMIT}
echo "RELEASE_VERSION:" ${RELEASE_VERSION}
echo "BUILD_TIME:" ${BUILD_TIME}

DEFAULT_ARCHS='amd64 arm64'
DEFAULT_TARGETS='linux darwin'
DEFAULT_WHAT='aws-k8s-tester cw-utils ec2-utils ecr-utils eks-utils etcd-utils s3-utils sts-utils'

ARCHS=${ARCHS:-$DEFAULT_ARCHS}
TARGETS=${TARGETS:-$DEFAULT_TARGETS}
WHAT=${WHAT:-$DEFAULT_WHAT}

echo ""
echo "Usage: \`make TARGETS='linux' WHAT='aws-k8s-tester cw-utils'\`"
echo "DEFAULT_ARCHS=$DEFAULT_ARCHS"
echo "DEFAULT_TARGETS=$DEFAULT_TARGETS"
echo "DEFAULT_WHAT=$DEFAULT_WHAT"
echo ""

mkdir -p ./bin

PACKAGE_NAME='github.com/aws/aws-k8s-tester'
for arch in ${ARCHS}; do
  for os in ${TARGETS}; do
    for bin in ${WHAT}; do
      echo "=== Building arch=${arch}, os=${os}, target=${bin} ==="
      CGO_ENABLED=0 \
        GOARCH=${arch} \
        GOOS=${os} \
        go build \
        -mod=mod -v \
        -ldflags "-s -w \
        -X ${PACKAGE_NAME}/version.GitCommit=${GIT_COMMIT} \
        -X ${PACKAGE_NAME}/version.ReleaseVersion=${RELEASE_VERSION} \
        -X ${PACKAGE_NAME}/version.BuildTime=${BUILD_TIME}" \
        -o ./bin/${bin}-${RELEASE_VERSION}-${os}-${arch} \
        ./cmd/${bin}
    done
  done
done

WHAT="k8s-tester"
PACKAGE_NAME='github.com/aws/aws-k8s-tester/k8s-tester'
pushd ./k8s-tester/cmd/k8s-tester
for arch in ${ARCHS}; do
  for os in ${TARGETS}; do
    for bin in ${WHAT}; do
      echo "=== Building arch=${arch}, os=${os}, target=${bin} ==="
      CGO_ENABLED=0 \
        GOARCH=${arch} \
        GOOS=${os} \
        go build \
        -v \
        -ldflags "-s -w \
        -X ${PACKAGE_NAME}/version.GitCommit=${GIT_COMMIT} \
        -X ${PACKAGE_NAME}/version.ReleaseVersion=${RELEASE_VERSION} \
        -X ${PACKAGE_NAME}/version.BuildTime=${BUILD_TIME}" \
        -o ../../../bin/${bin}-${RELEASE_VERSION}-${os}-${arch} \
        .
    done
  done
done
popd

echo ""
echo "Success! Your shiny new binaries are ready."
echo $(find ./bin -type f)
