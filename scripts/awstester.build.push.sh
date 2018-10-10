#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/awstester.build.push.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

$(aws ecr get-login --no-include-email --region us-west-2)

if [[ -z "${GIT_COMMIT}" ]]; then
  GIT_COMMIT=$(git rev-parse --short=12 HEAD || echo "GitNotFound")
fi

if [[ -z "${REGISTRY}" ]]; then
  REGISTRY=$(awstester ecr get-registry)
fi

echo "Building:" ${REGISTRY}/awstester:${GIT_COMMIT}

CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) \
  go build -v \
  -o ./awstester \
  ./cmd/awstester

docker build \
  --tag ${REGISTRY}/awstester:${GIT_COMMIT} \
  --file ./Dockerfile .

docker push ${REGISTRY}/awstester:${GIT_COMMIT}
