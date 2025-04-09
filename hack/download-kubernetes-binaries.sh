#!/usr/bin/env bash

set -o errexit
set -o nounset

BUNDLES=(
  "kubernetes-client"
  "kubernetes-test"
)

if [ "$#" -ne 3 ]; then
  echo >&2 "usage: $0 (KUBERNETES_MINOR_VERSION|latest) OS ARCH"
  exit 1
fi

if [ "$1" = "latest" ]; then
  RELEASE_MARKER="latest.txt"
else
  RELEASE_MARKER="latest-$1.txt"
fi

echo >&2 "Release marker: ${RELEASE_MARKER}"

OS="$2"
ARCH="$3"

function download_binaries() {
  local basePath=$1

  local KUBERNETES_VERSION=$(curl --silent "${basePath}/${RELEASE_MARKER}")

  echo "Kubernetes version: ${KUBERNETES_VERSION}"
  echo "${KUBERNETES_VERSION}" > kubernetes-version.txt

  for BUNDLE in ${BUNDLES[@]}; do
    echo >&2 "Downloading bundle: ${BUNDLE}"
    local TARBALL="${BUNDLE}.tar.gz"
    wget --quiet --output-document=${TARBALL} $basePath/${KUBERNETES_VERSION}/${BUNDLE}-${OS}-${ARCH}.tar.gz
    tar xzf ${TARBALL}
    rm ${TARBALL}
  done
}

if ! download_binaries https://storage.googleapis.com/kubernetes-release/release || true; then
  echo >&2 "binary download failed from release bucket, falling back to ci dev release"
  download_binaries https://storage.googleapis.com/k8s-release-dev/ci
fi
