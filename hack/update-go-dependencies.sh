#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

echo "Updating go modules..."
go get $(go list -f '{{if not (or .Main .Indirect)}}{{.Path}}{{end}}' -mod=mod -m all) && go mod tidy

echo "Updating kubetest2 image go version..."
MODULE_GO_VERSION=$(go list -m -f "{{if .Main}}{{.GoVersion}}{{end}}" | cut -d'.' -f1-2)
find . -type f -name Dockerfile -exec sed -i "s/\(GO_MINOR_VERSION\)=.*/\1=${MODULE_GO_VERSION}/g" {} +
