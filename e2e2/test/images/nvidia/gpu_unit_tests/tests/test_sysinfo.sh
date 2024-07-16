# Validate basic system configuration by comparing with expected config
#
setup_suite()
{
    source common.sh

    EC2_INSTANCE_TYPE=$(get_instance_type)
    data=test_sysinfo.sh.data/$EC2_INSTANCE_TYPE
    ACTUAL_RESULTS=`mktemp -t -d test_sysinfo.sh.actual-data.XXX`
    assert_not_equals "" "$ACTUAL_RESULTS"
    notify_trace_info "ACTUAL_RESULTS: $ACTUAL_RESULTS"

    if [ -n "$GENERATE_DATA" ]
    then
	echo "GENERATE_DATA is enabled..."
	mkdir -p $data
	function assert_data() {
	    generate_data "$@"
	}
    fi
}

teardown_suite()
{
    assert "test -z \"$GENERATE_DATA\"" "GENERATE_DATA was enabled, fail full suite"
    assert_gpu_unused
}


test_numa_topo_topo()
{
    assert_data $data/numa_topo.txt "grep . /sys/devices/system/node/node*/{cpulist,distance}" "Unexpected cpu topology"
}

test_nvidia_gpu_count()
{
    #Just for logging purposesclear
    assert_status_code 0 "nvidia-smi -q"
    assert_data $data/gpu_count.txt "nvidia-smi --query-gpu=name,index,pci.bus_id --format csv" "Unexpected gpu count"
}


test_nvidia_smi_topo()
{
    assert_data $data/nvidia_smi_topo.txt "nvidia-smi topo -m | grep GPU | cut -f 1-11" \
		  "Unexpected gpu topology, likely broken nvlinks"
}


test_nvidia_persistence_status()
{
    assert_data $data/nvidia_persistence_status.txt "nvidia-smi --query-gpu=name,pci.bus_id,persistence_mode --format=csv" \
		  "Unexpected perfistance status, likely system configuration issue"
}

test_nvidia_gpu_unused()
{
    assert_gpu_unused
}

test_nvidia_gpu_throttled()
{

    # https://docs.nvidia.com/deploy/nvml-api/group__nvmlClocksEventReasons.html#group__nvmlClocksEventReasons
    # The only  bit allowed is nvmlClocksEventReasonGpuIdle 0x0000000000000001LL
    filter="egrep -v -e '(0x0000000000000000|0x0000000000000001)'"
    cmd="nvidia-smi --query-gpu index,gpu_bus_id,gpu_uuid,clocks_throttle_reasons.active --format=csv,noheader"
    assert_status_code 1 "$cmd | $filter" "Throttled gpu detected, possible reason https://tt.amazon.com/P115211285"
}
