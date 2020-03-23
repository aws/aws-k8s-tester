#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/unit.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

make clean

echo "Running fmt tests..."
IGNORE_PKGS="(vendor)"
FORMATTABLE=$(find . -name \*.go | while read -r a; do echo "$(dirname "$a")/*.go"; done | sort | uniq | grep -vE "$IGNORE_PKGS" | sed "s|\./||g")
FMT=($FORMATTABLE)

function gofmt_pass {
	fmtRes=$(gofmt -l -s -d "${FMT[@]}")
	if [ -n "${fmtRes}" ]; then
		echo -e "gofmt checking failed:\\n${fmtRes}"
		exit 255
	fi
}

function govet_pass {
	vetRes=$(go vet ./...)
	if [ -n "${vetRes}" ]; then
		echo -e "govet checking failed:\\n${vetRes}"
		exit 255
	fi
}

gofmt_pass
govet_pass

echo "Running unit tests..."
go test -v ./eksconfig/...
go test -v -race ./eksconfig/...
go test -v ./ec2config/...
go test -v -race ./ec2config/...
