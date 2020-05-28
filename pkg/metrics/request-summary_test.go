package metrics

import (
	"fmt"
	"math"
	"sort"
	"testing"
	"time"
)

func TestRequestsSummary(t *testing.T) {
	ds := make(Durations, 20000)
	for i := 0; i < 20000; i++ {
		sign := 1
		if i%2 == 0 {
			sign = -1
		}
		dur := time.Second + time.Duration(sign*i)*time.Millisecond
		if dur < 0 {
			dur = 2 * time.Second
		}
		ds[20000-1-i] = dur
	}
	sort.Sort(ds)

	rs := RequestsSummary{
		SuccessTotal: 10,
		FailureTotal: 10,
		LatencyHistogram: HistogramBuckets([]HistogramBucket{
			{Scale: "milliseconds", LowerBound: 0, UpperBound: 0.5, Count: 0},
			{Scale: "milliseconds", LowerBound: 0.5, UpperBound: 1, Count: 2},
			{Scale: "milliseconds", LowerBound: 1, UpperBound: 2, Count: 0},
			{Scale: "milliseconds", LowerBound: 2, UpperBound: 4, Count: 0},
			{Scale: "milliseconds", LowerBound: 4, UpperBound: 8, Count: 0},
			{Scale: "milliseconds", LowerBound: 8, UpperBound: 16, Count: 8},
			{Scale: "milliseconds", LowerBound: 16, UpperBound: 32, Count: 0},
			{Scale: "milliseconds", LowerBound: 32, UpperBound: 64, Count: 100},
			{Scale: "milliseconds", LowerBound: 64, UpperBound: 128, Count: 0},
			{Scale: "milliseconds", LowerBound: 128, UpperBound: 256, Count: 0},
			{Scale: "milliseconds", LowerBound: 256, UpperBound: 512, Count: 20},
			{Scale: "milliseconds", LowerBound: 512, UpperBound: 1024, Count: 0},
			{Scale: "milliseconds", LowerBound: 1024, UpperBound: 2048, Count: 0},
			{Scale: "milliseconds", LowerBound: 2048, UpperBound: 4096, Count: 0},
			{Scale: "milliseconds", LowerBound: 4096, UpperBound: math.MaxFloat64, Count: 4},
		}),
		LantencyP50:   ds.PickLantencyP50(),
		LantencyP90:   ds.PickLantencyP90(),
		LantencyP99:   ds.PickLantencyP99(),
		LantencyP999:  ds.PickLantencyP999(),
		LantencyP9999: ds.PickLantencyP9999(),
	}
	fmt.Println(rs.JSON())
	fmt.Println(rs.Table())
}
