#!/usr/bin/env bash

set -o nounset
set -o errexit
set -o pipefail

# pip_versionsearch takes exactly 1 argument and returns its latest available version from the neuron pip repo
# usage: pip_versionsearch PACKAGE
pip_versionsearch() {
    local PACKAGE_INDEX_NAME=$(echo $1 | tr -s '_' '-')
    local PACKAGE_VERSION_NAME=$(echo $PACKAGE_INDEX_NAME | tr -s '-' '_')
    curl -s https://pip.repos.neuron.amazonaws.com/${PACKAGE_INDEX_NAME} | grep -o -G "${PACKAGE_VERSION_NAME}-[0-9\.]*+[a-f0-9]*" | sed "s/$PACKAGE_VERSION_NAME-//" | sort -V | tail -n 1 
}

# versionsearch takes exactly 1 argument and returns its latest available version from the neuron amd64 apt repo
# usage: versionsearch PACKAGE
versionsearch() {
    local PACKAGE_NAME=$1
    curl -s https://apt.repos.neuron.amazonaws.com/dists/focal/main/binary-amd64/Packages | grep -o "${PACKAGE_NAME}_[0-9\.]*-*[a-f0-9]*" | sed "s/${PACKAGE_NAME}_//" | sort -V | tail -n 1 
}

# update_arg ARG NEW_VALUE
update_arg() {
    local ARG=$1
    local NEW_VALUE=$2
    echo "setting $ARG to $NEW_VALUE"
    find . -type f -name Dockerfile -exec sed -i "s/${ARG}=.*/${ARG}=$NEW_VALUE/g" {} +
}

update_arg NEURONX_RUNTIME_LIB_VERSION $(versionsearch aws-neuronx-runtime-lib)
update_arg NEURONX_COLLECTIVES_LIB_VERSION $(versionsearch aws-neuronx-collectives)
update_arg NEURONX_TOOLS_VERSION $(versionsearch aws-neuronx-tools)
update_arg NEURONX_FRAMEWORK_VERSION $(pip_versionsearch torch-neuronx)
update_arg NEURONX_CC_VERSION $(pip_versionsearch neuronx-cc)
update_arg NEURONX_DISTRIBUTED_VERSION $(pip_versionsearch neuronx_distributed)