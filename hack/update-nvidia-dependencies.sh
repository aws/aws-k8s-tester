#!/usr/bin/env bash

# following from the last updated dependency:
# 1. get the latest release of aws-ofi-nccl
# 2. get the supported version of libnccl
# 3. get the latest correct cuda version used for libnccl

set -o nounset
set -o errexit
set -o pipefail

echo "Updating aws-ofi-nccl"
AWS_OFI_NCCL_TAG=$(curl -s https://api.github.com/repos/aws/aws-ofi-nccl/releases/latest | jq -r .tag_name | sed 's/^v//')
find . -type f -name Dockerfile -exec sed -i "s/AWS_OFI_NCCL_VERSION=.*/AWS_OFI_NCCL_VERSION=$AWS_OFI_NCCL_TAG/g" {} +

echo "Updating nccl"
LIB_NCCL_TAG=$(curl -s https://api.github.com/repos/aws/aws-ofi-nccl/releases/latest | jq -r .body | grep -oP '\[NCCL \K(\S*)(?=\])' | head -n 1 | sed 's/^v//')
find . -type f -name Dockerfile -exec sed -i "s/LIBNCCL_VERSION=.*/LIBNCCL_VERSION=$LIB_NCCL_TAG/g" {} +

echo "Updating nvbandwidth"
NVBANDWIDTH_TAG=$(curl -s https://api.github.com/repos/NVIDIA/nvbandwidth/releases/latest | jq -r .tag_name)
find . -type f -name Dockerfile -exec sed -i "s/NVBANDWIDTH_VERSION=.*/NVBANDWIDTH_VERSION=$NVBANDWIDTH_TAG/g" {} +

