# What

gpu_unit_tests is the unit tests for gpu enabled platforms. Idea is to create compact
set of tests which will cover most of performance critical aspects for gpu
platforms. Test designed to run on single instance.
# Usage

```
# Run tests
./unit_test
```

**Generate test data for new instance type**

Step 1: Copy the `gpu_unit_tests` folder to the EC2 instance where you want to generate the data.

Step 2:  Execute the following command in the `gpu_unit_tests` directory on the EC2 instance:
```
GENERATE_DATA=1 ./unit_test
```
Step 3:
Copy the files from `tests/test_sysinfo.sh.data` (e.g., `tests/test_sysinfo.sh.data/p3.2xlarge`) to your local repository.

Step 4:
Create PR with the new `tests/test_sysinfo.sh.data/xxx`

# Test list

-  test_sysinfo.sh :: Validate basic system configuration by comparing it with test config
  - test_numa_topo_topo :: check cpu/numa topology
  - test_nvidia_gpu_count :: fail if one of GPUs is broken or is not visiable
  - test_nvidia_fabric_status :: fail if fabric manager is not active
  - test_nvidia_smi_topo :: fail if nvidia-smi topology is differ
  - test_nvidia_persistence_status :: validate persistence state
  - test_nvidia_gpu_unused :: Check that no other process are using GPUs, fail is a signal system misconfiguration.


- 10_test_basic_cuda.sh :: Execute trivial cuda binaries, fail if cuda subsys is not healthy
  Use demo-suite binaries https://docs.nvidia.com/cuda/demo-suite/index.html and DCGM Diagnostics https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html#run-levels-and-tests 
  If this test suite fail this is a sign that cuda subsystem is not usable at all.
  Usually this is side effect of system misconfiguration (driver or fabric manager is not loaded)
  - test_01_device_query
  - test_02_vector_add
  - test_04_dcgm_diagnostics


