package stresser

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

func TestMetrics(t *testing.T) {
	start := time.Now().Add(-10 * time.Minute)
	readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
	readRequestLatencyMs.Observe(float64(time.Since(start) / time.Millisecond))
	readRequestLatencyMs.Observe(float64(time.Since(time.Now().Add(-time.Millisecond)) / time.Millisecond))

	readRequestsSuccessTotal.Inc()
	readRequestsSuccessTotal.Inc()

	buf := bytes.NewBuffer(nil)
	enc := expfmt.NewEncoder(buf, expfmt.FmtText)

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}

	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		switch *mf.Name {
		case "stresser_client_read_request_latency_milliseconds":
			for _, bucket := range mf.Metric[0].Histogram.Bucket {
				fmt.Println(*bucket.UpperBound, *bucket.CumulativeCount)
			}
			fmt.Println(*mf.Metric[0].Histogram.SampleCount)
		case "stresser_client_read_requests_success_total":
			gg := *mf.Metric[0].GetGauge()
			cnt := gg.GetValue()
			fmt.Println("count:", cnt)
			if cnt != 2 {
				t.Fatalf("stresser_client_read_requests_success_total expected 2, got %v", cnt)
			}
		}

		err := enc.Encode(mf)
		if err != nil {
			t.Fatal(err)
		}
	}

	body := buf.String()
	if !strings.Contains(body, `stresser_client_read_request_latency_milliseconds_bucket{le="+Inf"} 3`) {
		t.Fatalf("unexpected output:\n\n%s\n\n", body)
	}

	fmt.Println(body)
}
