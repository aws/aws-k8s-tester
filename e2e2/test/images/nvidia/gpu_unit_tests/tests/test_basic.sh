# Trivial cuda tests to validate that GPU it functional
# Use demu-suite binaries https://docs.nvidia.com/cuda/demo-suite/index.html 
# and DCGM Diagnostics https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html#run-levels-and-tests

setup_suite()
{
    source common.sh
    assert_gpu_unused
    DEMU_SUITE_DIR=${DEMU_SUITE_DIR:-$(realpath /usr/local/cuda/extras/demo_suite)}
}

teardown_suite()
{
    assert_gpu_unused
}

test_01_device_query()
{
    assert_status_code 0 "$DEMU_SUITE_DIR/deviceQuery"
}

test_02_vector_add()
{
    assert_status_code 0 "$DEMU_SUITE_DIR/vectorAdd"
}

test_03_bandwidth()
{
    assert_status_code 0 "$DEMU_SUITE_DIR/bandwidthTest --device=all --csv"
}

test_04_bus_grind()
{
    assert_status_code 0 "$DEMU_SUITE_DIR/busGrind -a"
}

test_05_dcgm_diagnostics()
{
    # https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/dcgm-diagnostics.html#run-levels-and-tests
    if [[ $EC2_INSTANCE_TYPE == g* ]]; then
        # The G series instance don't have nvlink and GPU p2p communication
        assert_status_code 0 'dcgmi diag -r "software,memory,memory_bandwidth"'
    else
        assert_status_code 0 "dcgmi diag -r 2"
    fi
}