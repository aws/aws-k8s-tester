#!/usr/bin/env bash

torchrun --nproc_per_node=2 --nnodes=1 pytorch_tests/testNeuronSingleAllReduce.py
torchrun --nproc_per_node=2 --nnodes=1 pytorch_tests/testNeuronParallelState.py
torchrun --nproc_per_node=2 --nnodes=1 pytorch_tests/testNeuronMlp.py