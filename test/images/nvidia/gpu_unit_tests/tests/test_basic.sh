# Trivial cuda tests to validate that GPU it functional
# Use demu-suite binaries https://docs.nvidia.com/cuda/demo-suite/index.html 
# and DCGM Diagnostics https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html#run-levels-and-tests

setup_suite()
{
    source common.sh
    assert_gpu_unused
    DEMO_SUITE_DIR=${DEMO_SUITE_DIR:-$(realpath /usr/local/cuda/extras/demo_suite)}
}

teardown_suite()
{
    assert_gpu_unused
}

test_01_device_query()
{
    assert_status_code 0 "$DEMO_SUITE_DIR/deviceQuery"
}

test_02_vector_add()
{
    assert_status_code 0 "$DEMO_SUITE_DIR/vectorAdd"
}

test_03_nvbandwidth()
{
    assert_status_code 0 "$DEMO_SUITE_DIR/nvbandwidth"
}

test_04_dcgm_diagnostics()
{
    # This test is not applicable for vGPU instance types.
    if is_vgpu; then
        skip "This test does not apply to vGPU instances (g6f.*, gr6f.*)"
    fi

    # https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html#run-levels-and-tests
    if [[ $EC2_INSTANCE_TYPE == g* ]]; then
        # The G series instance don't have nvlink and GPU p2p communication
        assert_status_code 0 'dcgmi diag -r "software,memory,memory_bandwidth"'
    else
        assert_status_code 0 "dcgmi diag -r 2"
    fi
}
