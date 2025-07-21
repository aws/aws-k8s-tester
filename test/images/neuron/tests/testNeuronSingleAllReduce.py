# Source: https://github.com/aws/deep-learning-containers/blob/master/test/dlc_tests/container_tests/bin/pytorch_tests/testNeuronSingleAllReduce
import os
import torch
import torch_xla.core.xla_model as xm
import torch_xla.distributed.xla_backend
import torch_xla.runtime as xr
torch.distributed.init_process_group('xla')
import torch_xla.distributed.xla_multiprocessing as xmp
os.environ["NEURON_RT_EXEC_TIMEOUT"] = "20"
os.environ["NCCL_DEBUG"] = "WARN"
os.environ["NCCL_DEBUG_SUBSYS"] = "ALL"
def _mp_fn():
  world_size = xr.world_size()
  device = xm.xla_device()
  rank = xr.global_ordinal()
  ones = torch.ones((2, 3))
  xones = ones.to(device)
  if world_size > 0:
    print("running all reduce")
    for i in range(0, 5):
        print(f'at iteration {i}, with local rank {rank}', flush=True)
        result = xm.all_reduce(xm.REDUCE_SUM, xones)
        result_cpu = result.cpu()
        #xm.mark_step()
        print(result_cpu, flush = True)
    expected = torch.ones((2,3))*world_size
    assert expected.allclose(result_cpu)
    print('PASS')
if __name__ == '__main__':
    _mp_fn()
    #xmp.spawn(_mp_fn, args=(),nprocs=2, join=True)
