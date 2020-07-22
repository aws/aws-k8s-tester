#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ hack/fmt.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

goimports -w ./cmd
goimports -w ./e2e
goimports -w ./ec2
goimports -w ./ec2config
goimports -w ./eks
goimports -w ./eksconfig
goimports -w ./pkg
goimports -w ./ssh
goimports -w ./version

gofmt -s -w ./cmd
gofmt -s -w ./e2e
gofmt -s -w ./ec2
gofmt -s -w ./ec2config
gofmt -s -w ./eks
gofmt -s -w ./eksconfig
gofmt -s -w ./pkg
gofmt -s -w ./ssh
gofmt -s -w ./version
