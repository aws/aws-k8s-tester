#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/aws-k8s-tester.build.release.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [[ -z "${GIT_COMMIT}" ]]; then
  GIT_COMMIT=$(git rev-parse --short=12 HEAD || echo "GitNotFound")
fi

if [[ -z "${RELEASE_VERSION}" ]]; then
  RELEASE_VERSION=$(git rev-parse --short=12 HEAD || echo "GitNotFound")
fi

if [[ -z "${BUILD_TIME}" ]]; then
  BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
fi

echo "GIT_COMMIT:" ${GIT_COMMIT}
echo "RELEASE_VERSION:" ${RELEASE_VERSION}
echo "BUILD_TIME:" ${BUILD_TIME}

mkdir -p ./bin

for os in linux darwin; do
  CGO_ENABLED=0 GOOS=${os} GOARCH=$(go env GOARCH) \
    go build -v \
    -ldflags "-s -w \
    -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
    -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
    -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
    -o ./bin/aws-k8s-tester-${RELEASE_VERSION}-${os}-$(go env GOARCH) \
    ./cmd/aws-k8s-tester
done

if [[ "${OSTYPE}" == "linux-gnu" ]]; then
  ./bin/aws-k8s-tester-${RELEASE_VERSION}-linux-$(go env GOARCH) version
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  ./bin/aws-k8s-tester-${RELEASE_VERSION}-darwin-$(go env GOARCH) version
fi

CGO_ENABLED=0 GOARCH=$(go env GOARCH) \
  go build -v \
  -ldflags "-s -w \
  -X github.com/aws/aws-k8s-tester/version.GitCommit=${GIT_COMMIT} \
  -X github.com/aws/aws-k8s-tester/version.ReleaseVersion=${RELEASE_VERSION} \
  -X github.com/aws/aws-k8s-tester/version.BuildTime=${BUILD_TIME}" \
  -o ./bin/aws-k8s-tester \
  ./cmd/aws-k8s-tester

./bin/aws-k8s-tester version

echo "Success!"
