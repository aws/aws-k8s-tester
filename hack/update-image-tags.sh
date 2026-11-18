#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

# All lookups below use anonymous access, so this script does not require AWS credentials.

ECR_PUBLIC_REGISTRY="public.ecr.aws"
EKS_CONTAINER_REGISTRY="602401143452.dkr.ecr.us-west-2.amazonaws.com"
EKS_CHARTS_RAW_URL="https://raw.githubusercontent.com/aws/eks-charts/master/stable"

# Names of images whose lookup failed; reported at the end so one broken upstream
# doesn't block the others from being updated.
FAILED=()

# get_registry_tags <REGISTRY> <REPOSITORY> <TOKEN_URL>
# Lists tags via the OCI distribution API using an anonymous bearer token.
get_registry_tags() {
    local REGISTRY=$1
    local REPOSITORY=$2
    local TOKEN_URL=$3
    local TOKEN
    TOKEN=$(curl -sSf "$TOKEN_URL" | jq -re '.token')
    curl -sSf -H "Authorization: Bearer $TOKEN" "https://$REGISTRY/v2/$REPOSITORY/tags/list" | jq -ce '.tags'
}

# update_image_uris <REPOSITORY> <IMAGE_TAG>
update_image_uris() {
    local REPOSITORY=$1
    local NEW_TAG=$2
    if [ -z "$NEW_TAG" ] || [ "$NEW_TAG" = "null" ]; then
        echo "ERROR: empty tag resolved for ${REPOSITORY}, skipping" >&2
        return 1
    fi
    echo "  ${REPOSITORY} -> ${NEW_TAG}"
    local PREFIX="image: ${REPOSITORY}"
    find ./test/manifests -type f -exec sed -i "s#$PREFIX:.*#$PREFIX:$NEW_TAG#g" {} +
}

update_nvidia_device_plugin() {
    local TAG
    TAG=$(get_registry_tags nvcr.io nvidia/k8s-device-plugin \
            "https://nvcr.io/proxy_auth?scope=repository:nvidia/k8s-device-plugin:pull" \
        | jq -r '[.[] | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))] | max_by(sub("^v"; "") | split(".") | map(tonumber))')
    update_image_uris nvcr.io/nvidia/k8s-device-plugin "$TAG"
}

update_neuron_device_plugin() {
    local REPOSITORY="neuron/neuron-device-plugin"
    local TAG
    TAG=$(get_registry_tags "$ECR_PUBLIC_REGISTRY" "$REPOSITORY" "https://${ECR_PUBLIC_REGISTRY}/token/" \
        | jq -r '[.[] | select(test("^[0-9]+(\\.[0-9]+)+$"))] | max_by(split(".") | map(tonumber))')
    update_image_uris "${ECR_PUBLIC_REGISTRY}/${REPOSITORY}" "$TAG"
}

update_efa_device_plugin() {
    # The EKS registry requires AWS credentials to list tags, so use the image tag
    # published in the official aws-efa-k8s-device-plugin Helm chart instead. This keeps
    # the -eksbuild.N suffix, which carries rebuilt images with security patches.
    local TAG
    TAG=$(curl -sSf "${EKS_CHARTS_RAW_URL}/aws-efa-k8s-device-plugin/values.yaml" \
        | awk '/^image:/{in_image=1; next} in_image && /^[^[:space:]]/{in_image=0} in_image && $1=="tag:"{gsub(/"/, "", $2); print $2; exit}')
    update_image_uris "${EKS_CONTAINER_REGISTRY}/eks/aws-efa-k8s-device-plugin" "$TAG"
}

for UPDATER in update_nvidia_device_plugin update_neuron_device_plugin update_efa_device_plugin; do
    echo "Running ${UPDATER}"
    # errexit is suspended inside an `if` condition, so failed lookups surface as an
    # empty/null tag, which update_image_uris rejects with a non-zero return.
    if ! "$UPDATER"; then
        echo "ERROR: ${UPDATER} failed" >&2
        FAILED+=("$UPDATER")
    fi
done

if [ ${#FAILED[@]} -gt 0 ]; then
    echo "ERROR: the following updaters failed: ${FAILED[*]}" >&2
    exit 1
fi
