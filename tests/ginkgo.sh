#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ tests/ginkgo.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

make clean
go install -v ./cmd/awstester



_GINKGO_TIMEOUT=10h
if [[ "${GINKGO_TIMEOUT}" ]]; then
  _GINKGO_TIMEOUT=${GINKGO_TIMEOUT}
fi

_GINKGO_VERBOSE=--ginkgo-verbose=true
if [[ "${GINKGO_VERBOSE}" ]]; then
  _GINKGO_VERBOSE=--ginkgo-verbose=${GINKGO_VERBOSE}
fi

pushd ${GOPATH}/src/github.com/aws/awstester/kubetest/

ginkgo \
  -r \
  -v \
  --failFast \
  --noColor \
  --progress \
  --race \
  --timeout=${_GINKGO_TIMEOUT} \
  -- \
  --ginkgo-command-timeout=${_GINKGO_TIMEOUT} \
  ${_GINKGO_VERBOSE}

popd
