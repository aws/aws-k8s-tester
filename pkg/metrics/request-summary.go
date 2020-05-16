package metrics

import (
	"encoding/json"
	"fmt"
)

// RequestsSummary represents request results.
type RequestsSummary struct {
	// SuccessTotal is the number of successful client requests.
	SuccessTotal float64 `json:"success-total" read-only:"true"`
	// FailureTotal is the number of failed client requests.
	FailureTotal float64 `json:"failure-total" read-only:"true"`
	// LatencyHistogram is the client requests latency histogram.
	LatencyHistogram HistogramBuckets `json:"latency-histogram,omitempty" read-only:"true"`
}

func (rs RequestsSummary) JSON() string {
	b, _ := json.Marshal(rs)
	return string(b)
}

func (rs RequestsSummary) Table() string {
	return fmt.Sprintf(`
SUCCESS TOTAL: %.2f
FAILURE TOTAL: %.2f

`,
		rs.SuccessTotal,
		rs.FailureTotal,
	) + rs.LatencyHistogram.Table()
}
