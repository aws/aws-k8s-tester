#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ hack/install.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

go install -v ./cmd/...
