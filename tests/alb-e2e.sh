#!/usr/bin/env bash
set -e

# TODO: move tests to "github.com/kubernetes-sigs/aws-alb-ingress-controller"?
# for now keep everything here for dependency management

if ! [[ "$0" =~ tests/alb-e2e.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

#########################################
# git checkout
mkdir -p ${GOPATH}/src/github.com/kubernetes-sigs/
cd ${GOPATH}/src/github.com/kubernetes-sigs/
git clone https://github.com/gyuho/aws-alb-ingress-controller.git

cd ${GOPATH}/src/github.com/kubernetes-sigs/aws-alb-ingress-controller
if [[ "${PULL_NUMBER}" ]]; then
  echo 'git fetching:' pull/${PULL_NUMBER}/head 'to test branch'
  git fetch origin pull/${PULL_NUMBER}/head:test
fi

# TODO: not work?
git remote -v

git checkout test
git log --pretty=oneline -10
#########################################

#########################################
if [[ -z "${REGISTRY}" ]]; then
  REGISTRY=607362164682.dkr.ecr.us-west-2.amazonaws.com
fi

# generate container image tag based on PR number
# https://github.com/kubernetes/test-infra/tree/master/prow#job-environment-variables
if [[ "${PULL_NUMBER}" ]]; then
  RELEASE_VERSION=v0.0.0-pr-${PULL_NUMBER}-${PULL_PULL_SHA}
fi

if [[ -z "${RELEASE_VERSION}" ]]; then
  RELEASE_VERSION=v0.0.0
fi

printenv
echo "AWS_SHARED_CREDENTIALS_FILE:" ${AWS_SHARED_CREDENTIALS_FILE}
echo 'REGISTRY:' ${REGISTRY}
echo 'RELEASE_VERSION:' ${RELEASE_VERSION}
#########################################



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
  --randomizeAllSpecs \
  --noColor \
  --progress \
  --race \
  --timeout=${_GINKGO_TIMEOUT} \
  -- \
  --ginkgo-command-timeout=${_GINKGO_TIMEOUT} \
  ${_GINKGO_VERBOSE}

popd
