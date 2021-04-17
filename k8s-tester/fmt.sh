#!/usr/bin/env bash
set -e

goimports -w ./nlb-hello-world
goimports -w ./tester

gofmt -s -w ./nlb-hello-world
gofmt -s -w ./tester
