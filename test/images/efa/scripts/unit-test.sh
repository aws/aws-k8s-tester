#!/usr/bin/env bash

set -eu

get_instance_type()
{

    local token=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null)

    if [ -n "$token" ]; then
        curl -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/instance-type
    else
        curl http://169.254.169.254/latest/meta-data/instance-type
    fi
}

get_expected_efa_device_count() 
{
    aws ec2 describe-instance-types --instance-type="$EC2_INSTANCE_TYPE" | jq -r '.InstanceTypes[].NetworkInfo.EfaInfo.MaximumEfaInterfaces'
}

EC2_INSTANCE_TYPE=${EC2_INSTANCE_TYPE:-$(get_instance_type)}
EXPECTED_EFA_DEVICE_COUNT=${EXPECTED_EFA_DEVICE_COUNT:-$(get_expected_efa_device_count)}

echo "Running test on a $EC2_INSTANCE_TYPE"

fi_info -p efa
DGRAM_ENDPOINT_COUNT=$(fi_info -p efa | grep 'type:\sFI_EP_DGRAM$' | wc -l)
if ! test $EXPECTED_EFA_DEVICE_COUNT -le $DGRAM_ENDPOINT_COUNT; then
    echo "Expected at least $EXPECTED_EFA_DEVICE_COUNT DGRAM endpoint(s) but found $DGRAM_ENDPOINT_COUNT"
    exit 1
else
    echo "Verified at least $EXPECTED_EFA_DEVICE_COUNT DGRAM endpoint(s) are available (found $DGRAM_ENDPOINT_COUNT)"
fi

RDM_ENDPOINT_COUNT=$(fi_info -p efa | grep 'type:\sFI_EP_RDM$' | wc -l)
if ! test $EXPECTED_EFA_DEVICE_COUNT -le $RDM_ENDPOINT_COUNT; then
    echo "Expected at least $EXPECTED_EFA_DEVICE_COUNT RDM endpoint(s) but found $RDM_ENDPOINT_COUNT"
    exit 1
else
    echo "Verified at least $EXPECTED_EFA_DEVICE_COUNT RDM endpoint(s) are available (found $RDM_ENDPOINT_COUNT)"
fi

echo "Success!"