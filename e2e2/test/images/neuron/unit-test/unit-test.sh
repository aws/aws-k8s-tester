#!/usr/bin/env bash

torchrun --nproc_per_node=2 --nnodes=1 unit-test/test-neuron-mlp.py
torchrun --nproc_per_node=2 --nnodes=1 unit-test/test-neuron-parallel-state.py
torchrun --nproc_per_node=2 --nnodes=1 unit-test/test-neuron-single-all-reduce.py