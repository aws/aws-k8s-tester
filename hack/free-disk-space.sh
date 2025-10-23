#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

# hack to free up disk space for build
# ref: https://github.com/easimon/maximize-build-space/blob/master/action.yml

# storage before
sudo df -h

sudo rm -rf \
  /usr/share/dotnet \
  /usr/local/lib/android \
  /opt/ghc \
  /opt/hostedtoolcache/CodeQL

docker image prune --all --force
docker builder prune -a

# storage after
sudo df -h
