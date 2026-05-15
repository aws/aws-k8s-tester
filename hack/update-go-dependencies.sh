#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

DEFAULT_PINNED_MODULES=()
IFS=' ' read -ra PINNED_MODULES <<< "${PINNED_GO_MODULES:-${DEFAULT_PINNED_MODULES[*]:-}}"

echo "Updating go modules..."
MODULES_TO_UPDATE=$(go list -e -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -mod=mod -m all)

if [[ ${#PINNED_MODULES[@]} -gt 0 ]]; then
  for mod in "${PINNED_MODULES[@]}"; do
    MODULES_TO_UPDATE=$(echo "$MODULES_TO_UPDATE" | grep -v "^${mod%%@*}$")
  done
fi

GO_GET_ERRORS=$(go get $MODULES_TO_UPDATE 2>&1) || true
if [[ -n "$GO_GET_ERRORS" ]]; then
  echo "WARNING: some modules failed to update:"
  echo "$GO_GET_ERRORS"
fi

if [[ ${#PINNED_MODULES[@]} -gt 0 ]]; then
  echo "Pinning modules..."
  go get "${PINNED_MODULES[@]}"
fi

go mod tidy

echo "Updating kubetest2 image go version..."
MODULE_GO_VERSION=$(go list -m -f "{{if .Main}}{{.GoVersion}}{{end}}" | cut -d'.' -f1-2)
find . -type f -name Dockerfile -exec sed -i "s/\(GO_MINOR_VERSION\)=.*/\1=${MODULE_GO_VERSION}/g" {} +
