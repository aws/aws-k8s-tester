#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ tests/ginkgo.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

make clean
go install -v ./cmd/awstester



pushd ${GOPATH}/src/github.com/aws/awstester/kubetest/

ginkgo \
  -r \
  -v \
  --failFast \
  --noColor \
  --progress \
  --race \
  --timeout=10h

popd

<<COMMENT
ginkgo \
  -r \
  -v \
  --failFast \
  --randomizeAllSpecs \
  --noColor \
  --progress \
  --race \
  --timeout=${_GINKGO_TIMEOUT} \
  -- \
  --ginkgo-command-timeout=${_GINKGO_TIMEOUT} \
  ${_GINKGO_VERBOSE}
COMMENT
