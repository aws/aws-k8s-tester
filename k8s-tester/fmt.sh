#!/usr/bin/env bash
set -e

# in case IDE/gopls doesn't work

goimports -w ./fluent-bit
gofmt -s -w ./fluent-bit

goimports -w ./jobs-echo
gofmt -s -w ./jobs-echo

goimports -w ./jobs-pi
gofmt -s -w ./jobs-pi

goimports -w ./nlb-hello-world
gofmt -s -w ./nlb-hello-world

goimports -w ./metrics-server
gofmt -s -w ./metrics-server

goimports -w ./tester
gofmt -s -w ./tester
