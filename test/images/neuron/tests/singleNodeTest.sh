#!/usr/bin/env bash

set -e

torchrun --nproc_per_node=2 --nnodes=1 tests/testNeuronSingleAllReduce.py
torchrun --nproc_per_node=2 --nnodes=1 tests/testNeuronParallelState.py
torchrun --nproc_per_node=2 --nnodes=1 tests/testNeuronMlp.py