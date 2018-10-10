#!/usr/bin/env bash
set -e

<<COMMENT
GO111MODULE=on go mod init
COMMENT

if ! [[ "$0" =~ scripts/update.vendor.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

if [[ $(go version) != "go version go1.11"* ]]; then
  echo 'expect Go 1.11+, got:' $(go version)
  exit 255
fi

GO111MODULE=on go mod tidy -v
GO111MODULE=on go mod vendor -v
