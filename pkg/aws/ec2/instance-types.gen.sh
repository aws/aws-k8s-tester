#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ instance-types.gen.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

go run ./instance-types
