#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ hack/fmt.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

goimports -w ./cmd
gofmt -s -w ./cmd

goimports -w ./client
gofmt -s -w ./client

goimports -w ./e2e
gofmt -s -w ./e2e

goimports -w ./ec2
gofmt -s -w ./ec2

goimports -w ./ec2config
gofmt -s -w ./ec2config

goimports -w ./eks
gofmt -s -w ./eks

goimports -w ./eksconfig
gofmt -s -w ./eksconfig

goimports -w ./k8s-tester
gofmt -s -w ./k8s-tester

goimports -w ./pkg
gofmt -s -w ./pkg

goimports -w ./ssh
gofmt -s -w ./ssh

goimports -w ./utils
gofmt -s -w ./utils

goimports -w ./version
gofmt -s -w ./version
