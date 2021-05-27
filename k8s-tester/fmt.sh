#!/usr/bin/env bash
set -e

goimports -w ./fluent-bit
gofmt -s -w ./fluent-bit

goimports -w ./jobs-pi
gofmt -s -w ./jobs-pi

goimports -w ./nlb-hello-world
gofmt -s -w ./nlb-hello-world

goimports -w ./tester
gofmt -s -w ./tester
