#!/usr/bin/env bash
set -e

# in case IDE/gopls doesn't work

goimports -w .
gofmt -s -w .

goimports -w ./cloudwatch-agent
gofmt -s -w ./cloudwatch-agent

goimports -w ./clusterloader
gofmt -s -w ./clusterloader

goimports -w ./cmd/k8s-tester
gofmt -s -w ./cmd/k8s-tester

goimports -w ./cmd/readme-gen
gofmt -s -w ./cmd/readme-gen

goimports -w ./configmaps
gofmt -s -w ./configmaps

goimports -w ./conformance
gofmt -s -w ./conformance

goimports -w ./csi-ebs
gofmt -s -w ./csi-ebs

goimports -w ./fluent-bit
gofmt -s -w ./fluent-bit

goimports -w ./helm
gofmt -s -w ./helm

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

goimports -w ./php-apache
gofmt -s -w ./php-apache

goimports -w ./stress
gofmt -s -w ./stress

goimports -w ./tester
gofmt -s -w ./tester

goimports -w ./version
gofmt -s -w ./version
