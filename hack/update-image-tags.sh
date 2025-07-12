#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

ECR_PUBLIC_REGISTRY="public.ecr.aws"
EKS_CONTAINER_REGISTRY="602401143452.dkr.ecr.us-west-2.amazonaws.com"

# get_ecr_image_tags <REGISTRY> <REPOSITORY>
# e.g. get_ecr_image_tags $ECR_PUBLIC_REGISTRY amazonlinux/amazonlinux
get_ecr_image_tags() {
    local REGISTRY=$1 
    local REPOSITORY=$2

    # Get ECR public token if image is from a public registry, otherwise use a private token
    # An authorization token is required for every ECR HTTP request
    if [ "$REGISTRY" = "$ECR_PUBLIC_REGISTRY" ]; then
        local TOKEN=$(aws ecr-public get-authorization-token --region us-east-1 --output=text --query 'authorizationData.authorizationToken')
        local AUTHORIZATION_TYPE="Bearer"
    else 
        local TOKEN=$(aws ecr get-authorization-token --output text --query 'authorizationData[].authorizationToken')
        local AUTHORIZATION_TYPE="Basic"
    fi


    if [ -n "${TOKEN}" ]; then
        curl -s -H "Authorization: ${AUTHORIZATION_TYPE} $TOKEN" "https://$REGISTRY/v2/$REPOSITORY/tags/list" | jq '.tags'
    else 
        echo ""
    fi
}

# update_image_uris REPOSITORY IMAGE_TAG
update_image_uris() {
    local REPOSITORY=$1
    local NEW_TAG=$2
    PREFIX="image: ${REPOSITORY}"
    find ./test/manifests -type f -exec sed -i "s#$PREFIX:.*#$PREFIX:$NEW_TAG#g" {} +
}

# update the nvidia k8s device plugin
echo "Updating Nvidia device plugin image"
NVIDIA_DEVICE_PLUGIN_TAG=$(curl -s 'https://catalog.ngc.nvidia.com/api/containers/images?orgName=nvidia&name=k8s-device-plugin&isPublic=true' | jq -r '.images | sort_by(.updatedDate) | reverse | map(select(.tag | test("^v[0-9]+.[0-9]+.[0-9]+$"))) | first | .tag')
update_image_uris nvcr.io/nvidia/k8s-device-plugin $NVIDIA_DEVICE_PLUGIN_TAG

# Below updates require authentication and should not fail fast
set +e

# update the neuron k8s device plugin
echo "Updating Neuron device plugin image"
NEURON_DEVICE_PLUGIN_REPOSITORY_NAME="neuron/neuron-device-plugin"
NEURON_DEVICE_PLUGIN_TAGS=$(get_ecr_image_tags $ECR_PUBLIC_REGISTRY $NEURON_DEVICE_PLUGIN_REPOSITORY_NAME)
if [ -n "$NEURON_DEVICE_PLUGIN_TAGS" ]; then
    LATEST_NEURON_DEVICE_PLUGIN_TAG=$(echo $NEURON_DEVICE_PLUGIN_TAGS | jq -r 'max_by(split(".") | map(tonumber))')
    update_image_uris "${ECR_PUBLIC_REGISTRY}/${NEURON_DEVICE_PLUGIN_REPOSITORY_NAME}" $LATEST_NEURON_DEVICE_PLUGIN_TAG
fi

# update the efa k8s device plugin
echo "Updating EFA device plugin image"
EFA_DEVICE_PLUGIN_REPOSITORY_NAME="eks/aws-efa-k8s-device-plugin"
EFA_DEVICE_PLUGIN_TAGS=$(get_ecr_image_tags $EKS_CONTAINER_REGISTRY $EFA_DEVICE_PLUGIN_REPOSITORY_NAME)
if [ -n "$EFA_DEVICE_PLUGIN_TAGS" ]; then
    LATEST_EFA_DEVICE_PLUGIN_TAG=$(echo $EFA_DEVICE_PLUGIN_TAGS | jq -r 'map(split("-") | .[0]) | max_by(sub("^v"; "") | split(".") | map(tonumber))')
    update_image_uris "${EKS_CONTAINER_REGISTRY}/${EFA_DEVICE_PLUGIN_REPOSITORY_NAME}" $LATEST_EFA_DEVICE_PLUGIN_TAG
fi