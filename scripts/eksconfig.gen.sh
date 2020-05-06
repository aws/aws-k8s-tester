#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/eksconfig.gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

rm -f eksconfig/README.md
go run eksconfig/gen/main.go
cat eksconfig/README.md

go install -v ./cmd/aws-k8s-tester
aws-k8s-tester eks create config --path ./eksconfig/default.yaml
rm -f ./eksconfig/default.kubectl.sh
rm -f ./eksconfig/default.ssh.sh
