#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset
set -o xtrace

if [ "$#" -eq 0 ]; then
  echo "Usage: $0 TEST_PATTERN [TEST_FLAGS]"
  exit 1
fi

TEST_PATTERN="${1}"
shift

# -count=1 is not a cacheable flag, so the test will always be executed
go test --timeout 0 -v -count=1 "${TEST_PATTERN}" ${@}
