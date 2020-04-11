#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/fmt.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

goimports -w ./cmd
goimports -w ./ec2
goimports -w ./ec2config
goimports -w ./eks
goimports -w ./eksconfig
goimports -w ./pkg
goimports -w ./ssh
goimports -w ./version

gofmt -w ./cmd
gofmt -w ./ec2
gofmt -w ./ec2config
gofmt -w ./eks
gofmt -w ./eksconfig
gofmt -w ./pkg
gofmt -w ./ssh
gofmt -w ./version
