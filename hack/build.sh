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

DEFAULT_WHAT='aws-k8s-tester cw-utils ec2-utils ecr-utils eks-utils etcd-utils s3-utils sts-utils'
DEFAULT_TARGETS='linux darwin'

PACKAGE_NAME='github.com/aws/aws-k8s-tester'
WHAT=${WHAT:-$DEFAULT_WHAT}
TARGETS=${TARGETS:-$DEFAULT_TARGETS}

echo ""
echo "Usage: \`make TARGET='linux' WHAT='aws-k8s-tester cw-utils'\`"
echo "DEFAULT_TARGETS=$DEFAULT_TARGETS"
echo "DEFAULT_WHAT=$DEFAULT_WHAT"
echo ""

mkdir -p ./bin

for os in ${TARGETS}; do
  for cmd in ${WHAT}; do
    echo "=== Building target=${cmd}, os=${os} ==="
    CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
      go build -mod=vendor -v \
      -ldflags "-s -w \
      -X ${PACKAGE_NAME}/version.GitCommit=${GIT_COMMIT} \
      -X ${PACKAGE_NAME}/version.ReleaseVersion=${RELEASE_VERSION} \
      -X ${PACKAGE_NAME}/version.BuildTime=${BUILD_TIME}" \
      -o ./bin/${cmd}-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
      ./cmd/${cmd}
  done
done

echo ""
echo "Success! Your shiny new binaries are ready."
echo "find ./bin -type f"
