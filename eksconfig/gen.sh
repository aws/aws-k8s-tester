#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ eksconfig/gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

rm -f eksconfig/README.md
go run eksconfig/gen/main.go
cat eksconfig/README.md
