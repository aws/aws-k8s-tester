#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

cd $(dirname $0)/../kubetest2

TEMP_DIR="$(mktemp -d)"
go mod vendor -o "${TEMP_DIR}"
if ! DIFF="$(diff -Naupr vendor ${TEMP_DIR})"; then
  echo "ERROR: the kubetest2/vendor directory is not up to date! You need to run 'go mod vendor' and commit the changes." >&2
  exit 1
fi
