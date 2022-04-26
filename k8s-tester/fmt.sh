#!/usr/bin/env bash
set -e

# in case IDE/gopls doesn't work

goimports -w .
gofmt -s -w .

goimports -w ./aqua
gofmt -s -w ./aqua

goimports -w ./armory
gofmt -s -w ./armory

goimports -w ./cloudwatch-agent
gofmt -s -w ./cloudwatch-agent

goimports -w ./clusterloader
gofmt -s -w ./clusterloader

goimports -w ./cmd/k8s-tester
gofmt -s -w ./cmd/k8s-tester

goimports -w ./cmd/readme-gen
gofmt -s -w ./cmd/readme-gen

goimports -w ./cni
gofmt -s -w ./cni

goimports -w ./configmaps
gofmt -s -w ./configmaps

goimports -w ./conformance
gofmt -s -w ./conformance

goimports -w ./csi-efs
gofmt -s -w ./csi-efs

goimports -w ./csi-ebs
gofmt -s -w ./csi-ebs

goimports -w ./csrs
gofmt -s -w ./csrs

goimports -w ./falco
gofmt -s -w ./falco

goimports -w ./falcon
gofmt -s -w ./falcon

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

goimports -w ./nlb-guestbook
gofmt -s -w ./nlb-guestbook

goimports -w ./nlb-hello-world
gofmt -s -w ./nlb-hello-world

goimports -w ./php-apache
gofmt -s -w ./php-apache

goimports -w ./secrets
gofmt -s -w ./secrets

goimports -w ./splunk
gofmt -s -w ./splunk

goimports -w ./stress
gofmt -s -w ./stress

goimports -w ./sysdig
gofmt -s -w ./sysdig

goimports -w ./tester
gofmt -s -w ./tester

goimports -w ./vault
gofmt -s -w ./vault

goimports -w ./tester
gofmt -s -w ./tester

goimports -w ./wordpress
gofmt -s -w ./wordpress
