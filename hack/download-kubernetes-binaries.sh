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

KUBERNETES_VERSION=$(curl --silent "https://storage.googleapis.com/kubernetes-release/release/${RELEASE_MARKER}")

echo "Kubernetes version: ${KUBERNETES_VERSION}"
echo "${KUBERNETES_VERSION}" > kubernetes-version.txt

for BUNDLE in ${BUNDLES[@]}; do
  echo >&2 "Downloading bundle: ${BUNDLE}"
  TARBALL="${BUNDLE}.tar.gz"
  wget --quiet --output-document=${TARBALL} https://storage.googleapis.com/kubernetes-release/release/${KUBERNETES_VERSION}/${BUNDLE}-${OS}-${ARCH}.tar.gz
  tar xzf ${TARBALL}
  rm ${TARBALL}
done

