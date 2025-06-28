#!/usr/bin/env bash

# following from the last updated dependency:
# 1. get the latest release of aws-ofi-nccl
# 2. get the supported version of libnccl
# 3. get the latest correct cuda version used for libnccl

set -o nounset
set -o errexit
set -o pipefail

AWS_OFI_NCCL_TAG=$(curl -s https://api.github.com/repos/aws/aws-ofi-nccl/releases/latest | jq -r .tag_name | sed 's/^v//')
LIB_NCCL_TAG=$(curl -s https://api.github.com/repos/aws/aws-ofi-nccl/releases/latest | jq -r .body | grep -oP '\[NCCL \K(\S*)(?=\])' | head -n 1 | sed 's/^v//')

find . -type f -name Dockerfile -exec sed -i "s/AWS_OFI_NCCL_VERSION=.*/AWS_OFI_NCCL_VERSION=$AWS_OFI_NCCL_TAG/g" {} +
find . -type f -name Dockerfile -exec sed -i "s/LIBNCCL_VERSION=.*/LIBNCCL_VERSION=$LIB_NCCL_TAG/g" {} +

CUDA=$(curl -s https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2204/x86_64/Packages  | grep 'Package: libnccl2' -A 1 | grep $LIB_NCCL_TAG | grep -oP 'Version: \K(\S*)(?=)' | sort -r | head -n 1 | sed "s/$LIB_NCCL_TAG+cuda//")
CUDA_VERSION_PARTS=($(echo $CUDA | tr -s '.' ' '))
CUDA_MAJOR_VERSION=${CUDA_VERSION_PARTS[0]}
CUDA_MINOR_VERSION=${CUDA_VERSION_PARTS[1]}

find . -type f -name Dockerfile -exec sed -i "s/CUDA_MINOR_VERSION=.*/CUDA_MINOR_VERSION=$CUDA_MINOR_VERSION/g" {} +
find . -type f -name Dockerfile -exec sed -i "s/CUDA_MAJOR_VERSION=.*/CUDA_MAJOR_VERSION=$CUDA_MAJOR_VERSION/g" {} +
