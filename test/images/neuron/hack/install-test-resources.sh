#!/bin/bash

set -o pipefail
set -o nounset
set -o errexit

USER_DIR=${1:-"/root"}
# attempt to cache dataset to avoid runtime download. 
# needs to match https://github.com/pytorch/vision/blob/c0331c5e2933c621db9a44623f4f3981fe2342e0/torchvision/datasets/mnist.py#L42
MNIST_RESOURCES=("train-images-idx3-ubyte.gz" "train-labels-idx1-ubyte.gz" "t10k-images-idx3-ubyte.gz" "t10k-labels-idx1-ubyte.gz")
for i in {0..1}; do 
    # we need to populate data for each rank, and we currently always run with 2
    DOWNLOAD_DIR="${USER_DIR}/MNIST_DATA_train/${i}/MNIST/raw"
    mkdir -p "$DOWNLOAD_DIR"
    for RESOURCE in ${MNIST_RESOURCES[@]}; do
        DEST_FILE="${DOWNLOAD_DIR}/${RESOURCE}"
        SOURCE_URL="https://ossci-datasets.s3.amazonaws.com/mnist/${RESOURCE}"
        echo "Downloading ${SOURCE_URL} to ${DEST_FILE}"
        curl -o "$DEST_FILE" "$SOURCE_URL"
    done 
done