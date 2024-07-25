#!/usr/bin/env bash
set -o pipefail
set -e

ulimit -n 65535

export FI_EFA_USE_DEVICE_RDMA=1
export FI_PROVIDER=efa
export FI_EFA_FORK_SAFE=1
# MPI runs on EKS
export CCOM_SOCKET_IFNAME=eth0
NODELIST=`python e2e-test/nodelist-helper.py`
HOSTS=(${NODELIST//\ / })
NODEID=$OMPI_COMM_WORLD_RANK
NTASKS=$OMPI_COMM_WORLD_SIZE


export NEURON_COMPILE_CACHE_URL="/shared/neuron_cache" # Place cache on shared storage to reduce redundant compilations
export PROCESSES_PER_NODE=$NEURON_CORES_PER_NODE
export MASTER_ADDR=${HOSTS[0]}
export MASTER_PORT=12355

export DISTRIBUTED_ARGS="--nproc_per_node $PROCESSES_PER_NODE --nnodes $NTASKS --node_rank $NODEID --master_addr $MASTER_ADDR --master_port $MASTER_PORT"
echo $DISTRIBUTED_ARGS
NEURON_CC_FLAGS="--model-type transformer"

torchrun $DISTRIBUTED_ARGS e2e-test/bert-train.py