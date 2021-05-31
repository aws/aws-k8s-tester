#!/usr/bin/env bash
set -e

<<COMMENT
go mod init
go mod vendor -v
COMMENT

go mod init || true
go mod tidy -v
