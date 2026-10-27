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

# the legacy kubernetes-release GCS bucket is no longer updated, so released
# versions are resolved from dl.k8s.io. minor versions that have not been
# released yet only have a marker in the CI bucket.
RELEASE_BASE_PATH="https://dl.k8s.io/release"
CI_BASE_PATH="https://storage.googleapis.com/k8s-release-dev/ci"

if [ "$1" = "latest" ]; then
  RELEASE_MARKER="latest.txt"
  CI_MARKER="latest.txt"
else
  RELEASE_MARKER="stable-$1.txt"
  CI_MARKER="latest-$1.txt"
fi

OS="$2"
ARCH="$3"

function download_binaries() {
  local basePath=$1
  local marker=$2

  echo >&2 "Release marker: ${basePath}/${marker}"
  local KUBERNETES_VERSION
  if ! KUBERNETES_VERSION=$(curl --silent --fail --location "${basePath}/${marker}"); then
    echo >&2 "release marker not found: ${basePath}/${marker}"
    return 1
  fi

  echo "Kubernetes version: ${KUBERNETES_VERSION}"
  echo "${KUBERNETES_VERSION}" > kubernetes-version.txt

  for BUNDLE in ${BUNDLES[@]}; do
    echo >&2 "Downloading bundle: ${BUNDLE}"
    local TARBALL="${BUNDLE}.tar.gz"
    if ! wget --quiet --output-document=${TARBALL} $basePath/${KUBERNETES_VERSION}/${BUNDLE}-${OS}-${ARCH}.tar.gz; then
      return 1
    fi
    tar xzf ${TARBALL}
    rm ${TARBALL}
  done
}

if ! download_binaries "${RELEASE_BASE_PATH}" "${RELEASE_MARKER}"; then
  echo >&2 "binary download failed from release bucket, falling back to ci dev release"
  download_binaries "${CI_BASE_PATH}" "${CI_MARKER}"
fi
