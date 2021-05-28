#!/usr/bin/env bash
set -e

# in case IDE/gopls doesn't work

goimports -w .
gofmt -s -w .

goimports -w ./cmd/k8s-tester
gofmt -s -w ./cmd/k8s-tester

goimports -w ./cloudwatch-agent
gofmt -s -w ./cloudwatch-agent

goimports -w ./fluent-bit
gofmt -s -w ./fluent-bit

goimports -w ./jobs-echo
gofmt -s -w ./jobs-echo

goimports -w ./jobs-pi
gofmt -s -w ./jobs-pi

goimports -w ./kubernetes-dashboard
gofmt -s -w ./kubernetes-dashboard

goimports -w ./metrics-server
gofmt -s -w ./metrics-server

goimports -w ./nlb-hello-world
gofmt -s -w ./nlb-hello-world

goimports -w ./tester
gofmt -s -w ./tester
