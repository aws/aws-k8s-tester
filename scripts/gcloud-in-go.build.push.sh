#!/usr/bin/env bash
set -e

if ! [[ "$0" =~ scripts/gcloud-in-go.build.push.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

$(aws ecr get-login --no-include-email --region us-west-2)

if [[ -z "${GIT_COMMIT}" ]]; then
  GIT_COMMIT=$(git rev-parse --short=12 HEAD || echo "GitNotFound")
fi

_AWS_REGION=us-west-2
if [[ "${AWS_REGION}" ]]; then
  _AWS_REGION=${AWS_REGION}
fi

if [[ -z "${REGISTRY}" ]]; then
  REGISTRY=$(awstester ecr --region=${_AWS_REGION} get-registry)
fi

echo "Building:" ${REGISTRY}/gcloud-in-go:${GIT_COMMIT}

docker build \
  --tag ${REGISTRY}/gcloud-in-go:${GIT_COMMIT} \
  --file ./scripts/gcloud-in-go.Dockerfile .

docker push ${REGISTRY}/gcloud-in-go:${GIT_COMMIT}
