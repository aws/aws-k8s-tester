#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

# update the nvidia k8s device plugin

NVIDIA_DEVICE_PLUGIN=$(curl -s 'https://catalog.ngc.nvidia.com/api/containers/images?orgName=nvidia&name=k8s-device-plugin&isPublic=true' | jq -r '.images | sort_by(.updatedDate) | reverse | map(select(.tag | test("^v[0-9]+.[0-9]+.[0-9]+$"))) | first | .tag')
PREFIX="image: nvcr.io/nvidia/k8s-device-plugin"
find ./test/manifests -type f -exec sed -i "s#$PREFIX:.*#$PREFIX:$NVIDIA_DEVICE_PLUGIN#g" {} +

# update the neuron k8s device plugin
# TODO

# update the efa k8s device plugin
# TODO
