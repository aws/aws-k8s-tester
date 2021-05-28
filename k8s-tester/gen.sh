#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

pushd cmd/readme-gen
go run ./main.go
popd
