package metrics

import (
	"encoding/json"
	"fmt"
	"time"
)

// RequestsSummary represents request results.
type RequestsSummary struct {
	// SuccessTotal is the number of successful client requests.
	SuccessTotal float64 `json:"success-total" read-only:"true"`
	// FailureTotal is the number of failed client requests.
	FailureTotal float64 `json:"failure-total" read-only:"true"`
	// LatencyHistogram is the client requests latency histogram.
	LatencyHistogram HistogramBuckets `json:"latency-histogram" read-only:"true"`

	// LantencyP50 is the 50-percentile latency.
	LantencyP50 time.Duration `json:"latency-p50" read-only:"true"`
	// LantencyP90 is the 90-percentile latency.
	LantencyP90 time.Duration `json:"latency-p90" read-only:"true"`
	// LantencyP99 is the 99-percentile latency.
	LantencyP99 time.Duration `json:"latency-p99" read-only:"true"`
	// LantencyP999 is the 99.9-percentile latency.
	LantencyP999 time.Duration `json:"latency-p99.9" read-only:"true"`
	// LantencyP9999 is the 99.99-percentile latency.
	LantencyP9999 time.Duration `json:"latency-p99.99" read-only:"true"`
}

func (rs RequestsSummary) JSON() string {
	b, _ := json.Marshal(rs)
	return string(b)
}

func (rs RequestsSummary) Table() string {
	return fmt.Sprintf(`
        TOTAL: %.2f
SUCCESS TOTAL: %.2f
FAILURE TOTAL: %.2f

`,
		rs.SuccessTotal+rs.FailureTotal,
		rs.SuccessTotal,
		rs.FailureTotal,
	) +
		rs.LatencyHistogram.Table() +
		fmt.Sprintf(`
   50-percentile Latency: %s
   90-percentile Latency: %s
   99-percentile Latency: %s
 99.9-percentile Latency: %s
99.99-percentile Latency: %s

`,
			rs.LantencyP50,
			rs.LantencyP90,
			rs.LantencyP99,
			rs.LantencyP999,
			rs.LantencyP9999,
		)
}

type Durations []time.Duration

func (ds Durations) Len() int           { return len(ds) }
func (ds Durations) Less(i, j int) bool { return ds[i] < ds[j] }
func (ds Durations) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }

// PickLantencyP50 returns the latency assuming durations are already sorted.
func (ds Durations) PickLantencyP50() time.Duration {
	n := len(ds)
	if n == 0 {
		return time.Duration(0)
	}
	if n == 1 {
		return ds[0]
	}

	idx := n / 2
	return ds[idx]
}

// PickLantencyP90 returns the latency assuming durations are already sorted.
func (ds Durations) PickLantencyP90() time.Duration {
	n := len(ds)
	if n == 0 {
		return time.Duration(0)
	}
	if n == 1 {
		return ds[0]
	}

	idx := n * 90 / 100
	if idx >= n {
		return ds[n-1]
	}
	return ds[idx]
}

// PickLantencyP99 returns the latency assuming durations are already sorted.
func (ds Durations) PickLantencyP99() time.Duration {
	n := len(ds)
	if n == 0 {
		return time.Duration(0)
	}
	if n == 1 {
		return ds[0]
	}

	idx := n * 99 / 100
	if idx >= n {
		return ds[n-1]
	}
	return ds[idx]
}

// PickLantencyP999 returns the latency assuming durations are already sorted.
func (ds Durations) PickLantencyP999() time.Duration {
	n := len(ds)
	if n == 0 {
		return time.Duration(0)
	}
	if n == 1 {
		return ds[0]
	}

	idx := n * 999 / 1000
	if idx >= n {
		return ds[n-1]
	}
	return ds[idx]
}

// PickLantencyP9999 returns the latency assuming durations are already sorted.
func (ds Durations) PickLantencyP9999() time.Duration {
	n := len(ds)
	if n == 0 {
		return time.Duration(0)
	}
	if n == 1 {
		return ds[0]
	}

	idx := n * 9999 / 10000
	if idx >= n {
		return ds[n-1]
	}
	return ds[idx]
}
