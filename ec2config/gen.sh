#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ ec2config/gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

rm -f ec2config/README.md
go run ec2config/gen/main.go
cat ec2config/README.md
