#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ tests/unit.sh ]]; then
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

function govet_shadow_pass {
	fmtpkgs=$(for a in "${FMT[@]}"; do dirname "$a"; done | sort | uniq | grep -v "\\.")
	fmtpkgs=($fmtpkgs)
	vetRes=$(go tool vet -all -shadow "${fmtpkgs[@]}" 2>&1 | grep -v '/gw/' || true)
	if [ -n "${vetRes}" ]; then
		echo -e "govet -all -shadow checking failed:\\n${vetRes}"
		exit 255
	fi
}

gofmt_pass
govet_pass
govet_shadow_pass

echo "Running unit tests..."
go test -v ./eksconfig/...
go test -v -race ./eksconfig/...
go test -v ./internal/...
go test -v -race ./internal/...
go test -v ./pkg/...
go test -v -race ./pkg/...
