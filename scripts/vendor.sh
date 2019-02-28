#!/usr/bin/env bash
set -e

<<COMMENT
GO111MODULE=on go mod init
COMMENT

if ! [[ "$0" =~ scripts/vendor.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

GO111MODULE=on go mod tidy -v
GO111MODULE=on go mod vendor -v
