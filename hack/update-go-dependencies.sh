#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

DEFAULT_PINNED_MODULES=(
  "sigs.k8s.io/karpenter@v1.9.0"
  "k8s.io/api@v0.35.5"
  "k8s.io/apimachinery@v0.35.5"
  "k8s.io/client-go@v0.35.5"
  "k8s.io/cli-runtime@v0.35.5"
  "k8s.io/cloud-provider-aws@v1.35.2"
  "sigs.k8s.io/controller-runtime@v0.23.3"
)
IFS=' ' read -ra PINNED_MODULES <<< "${PINNED_GO_MODULES:-${DEFAULT_PINNED_MODULES[*]}}"

PINNED_PATHS=()
for mod in "${PINNED_MODULES[@]}"; do
  PINNED_PATHS+=("${mod%%@*}")
done

echo "Updating go modules..."
MODULES_TO_UPDATE=$(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -mod=mod -m all)
for pinned in "${PINNED_PATHS[@]}"; do
  MODULES_TO_UPDATE=$(echo "$MODULES_TO_UPDATE" | grep -v "^${pinned}$")
done
go get $MODULES_TO_UPDATE

echo "Pinning modules..."
go get "${PINNED_MODULES[@]}"

go mod tidy -e

echo "Updating kubetest2 image go version..."
MODULE_GO_VERSION=$(go list -m -f "{{if .Main}}{{.GoVersion}}{{end}}" | cut -d'.' -f1-2)
find . -type f -name Dockerfile -exec sed -i "s/\(GO_MINOR_VERSION\)=.*/\1=${MODULE_GO_VERSION}/g" {} +
