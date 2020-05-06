#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/ec2config.gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

rm -f ec2config/README.md
go run ec2config/gen/main.go
cat ec2config/README.md

go install -v ./cmd/aws-k8s-tester
ec2-utils create config --path ./ec2config/default.yaml
rm -f ./ec2config/default.ssh.sh
