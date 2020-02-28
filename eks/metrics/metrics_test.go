package metrics

import (
	"strings"
	"testing"
)

// $ curl -sL http://localhost:8080/metrics | grep storage_

const outputEncrypt = `
# HELP apiserver_storage_data_key_generation_duration_seconds Latencies in seconds of data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_duration_seconds histogram
apiserver_storage_data_key_generation_duration_seconds_bucket{le="5e-06"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="1e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="2e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="4e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="8e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00016"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00032"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00064"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00128"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00256"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00512"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.01024"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.02048"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.04096"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="+Inf"} 0
apiserver_storage_data_key_generation_duration_seconds_sum 0
apiserver_storage_data_key_generation_duration_seconds_count 0
# HELP apiserver_storage_data_key_generation_failures_total Total number of failed data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_failures_total counter
apiserver_storage_data_key_generation_failures_total 0
# HELP apiserver_storage_data_key_generation_latencies_microseconds (Deprecated) Latencies in microseconds of data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_latencies_microseconds histogram
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="5"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="10"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="20"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="40"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="80"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="160"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="320"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="640"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="1280"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="2560"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="5120"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="10240"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="20480"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="40960"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="+Inf"} 0
apiserver_storage_data_key_generation_latencies_microseconds_sum 0
apiserver_storage_data_key_generation_latencies_microseconds_count 0
# HELP apiserver_storage_envelope_transformation_cache_misses_total Total number of cache misses while accessing key decryption key(KEK).
# TYPE apiserver_storage_envelope_transformation_cache_misses_total counter
apiserver_storage_envelope_transformation_cache_misses_total 33
# HELP apiserver_storage_transformation_duration_seconds Latencies in seconds of value transformation operations.
# TYPE apiserver_storage_transformation_duration_seconds histogram
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="5e-06"} 13
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="1e-05"} 32
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="2e-05"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="4e-05"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="8e-05"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00016"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00032"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00064"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00128"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00256"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.00512"} 34
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.01024"} 62
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.02048"} 66
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="0.04096"} 67
apiserver_storage_transformation_duration_seconds_bucket{transformation_type="from_storage",le="+Inf"} 67
apiserver_storage_transformation_duration_seconds_sum{transformation_type="from_storage"} 0.31080741999999995
apiserver_storage_transformation_duration_seconds_count{transformation_type="from_storage"} 67
# HELP apiserver_storage_transformation_latencies_microseconds (Deprecated) Latencies in microseconds of value transformation operations.
# TYPE apiserver_storage_transformation_latencies_microseconds histogram
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="5"} 24
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="10"} 32
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="20"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="40"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="80"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="160"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="320"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="640"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="1280"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="2560"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="5120"} 34
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="10240"} 62
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="20480"} 66
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="40960"} 67
apiserver_storage_transformation_latencies_microseconds_bucket{transformation_type="from_storage",le="+Inf"} 67
apiserver_storage_transformation_latencies_microseconds_sum{transformation_type="from_storage"} 310823
apiserver_storage_transformation_latencies_microseconds_count{transformation_type="from_storage"} 67
`
const outputNoEncrypt = `
# HELP apiserver_storage_data_key_generation_duration_seconds Latencies in seconds of data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_duration_seconds histogram
apiserver_storage_data_key_generation_duration_seconds_bucket{le="5e-06"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="1e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="2e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="4e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="8e-05"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00016"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00032"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00064"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00128"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00256"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.00512"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.01024"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.02048"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="0.04096"} 0
apiserver_storage_data_key_generation_duration_seconds_bucket{le="+Inf"} 0
apiserver_storage_data_key_generation_duration_seconds_sum 0
apiserver_storage_data_key_generation_duration_seconds_count 0
# HELP apiserver_storage_data_key_generation_failures_total Total number of failed data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_failures_total counter
apiserver_storage_data_key_generation_failures_total 0
# HELP apiserver_storage_data_key_generation_latencies_microseconds (Deprecated) Latencies in microseconds of data encryption key(DEK) generation operations.
# TYPE apiserver_storage_data_key_generation_latencies_microseconds histogram
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="5"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="10"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="20"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="40"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="80"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="160"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="320"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="640"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="1280"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="2560"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="5120"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="10240"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="20480"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="40960"} 0
apiserver_storage_data_key_generation_latencies_microseconds_bucket{le="+Inf"} 0
apiserver_storage_data_key_generation_latencies_microseconds_sum 0
apiserver_storage_data_key_generation_latencies_microseconds_count 0
# HELP apiserver_storage_envelope_transformation_cache_misses_total Total number of cache misses while accessing key decryption key(KEK).
# TYPE apiserver_storage_envelope_transformation_cache_misses_total counter
apiserver_storage_envelope_transformation_cache_misses_total 0
`

func TestMetrics(t *testing.T) {
	mfs1, err := parse(strings.NewReader(outputNoEncrypt))
	if err != nil {
		t.Fatal(err)
	}
	if mfs1["apiserver_storage_envelope_transformation_cache_misses_total"].Metric[0].GetCounter().GetValue() != 0 {
		t.Fatalf("expected 0 apiserver_storage_envelope_transformation_cache_misses_total, got %v", mfs1["apiserver_storage_envelope_transformation_cache_misses_total"].Metric[0].GetCounter().GetValue())
	}

	mfs2, err := parse(strings.NewReader(outputEncrypt))
	if err != nil {
		t.Fatal(err)
	}
	if mfs2["apiserver_storage_envelope_transformation_cache_misses_total"].Metric[0].GetCounter().GetValue() != 33 {
		t.Fatalf("expected 33 apiserver_storage_envelope_transformation_cache_misses_total, got %v", mfs2["apiserver_storage_envelope_transformation_cache_misses_total"].Metric[0].GetCounter().GetValue())
	}
}
