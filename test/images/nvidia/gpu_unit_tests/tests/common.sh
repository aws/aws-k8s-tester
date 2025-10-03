#!/bin/bash

get_instance_type()
{
    # Retrieve instance metadata: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html#instance-metadata-retrieval-examples
    [ -n "$FORCE_INSTANCE_TYPE" ] && echo $FORCE_INSTANCE_TYPE

    local token=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null)

    if [ -n "$token" ]; then
        curl -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/instance-type
    else
        curl http://169.254.169.254/latest/meta-data/instance-type
    fi
}

assert_gpu_unused()
{
    cmd="nvidia-smi --query-compute-apps timestamp,gpu_bus_id,gpu_uuid,pid,name,used_memory --format csv,noheader"
    assert_equals "" "`$cmd`" "gpu is busy by other task, system misconfig?"
}

_assert_data()
{
    local expected="$1"
    local cmd="$2"
    local message="${3:-}"
    local cmd_out="$ACTUAL_RESULTS/$(basename $expected)"
    [[ -z $message ]] || message="$message\n"

    eval "$cmd" > $cmd_out
    diff_cmd="diff -up $expected $cmd_out"
    diff_out="`$diff_cmd`"

    notify_trace_dbg "_assert_data $diff_cmd, out: $diff_out"
    if [ -n "$diff_out" ]
    then
	fail "$message test data value diff:\n$diff_out"
    fi
}

assert_data() {
    _assert_data "$1" "$2" "$3"
}

generate_data()
{
    local expected="$1"
    local cmd="$2"
    local msg="$3"
    local cmd_out="$ACTUAL_RESULTS/$(basename $expected)"

    eval "$cmd" > $expected
    _assert_data "$expected" "$cmd" "$msg"
}

skip_for_vgpu_instances() {
    local test_name="$1"
    local instance_type="$(get_instance_type)"
    case "$instance_type" in
        g6f*) echo "skipping $test_name: no current support for instance_type: $instance_type"
            exit 0;;
        gr6f*) echo "skipping $test_name: no current support for instance_type: $instance_type"
            exit 0;;
    esac
}

function is_vgpu()
{
  local instance_type="$(get_instance_type)"
  case "${instance_type}" in
    g6f.*|gr6f.*) return ;;
    *) return 1 ;;  # Not supported
  esac
}
