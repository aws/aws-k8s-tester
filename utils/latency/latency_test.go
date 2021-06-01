package latency

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSummary(t *testing.T) {
	ds1 := make(Durations, 20000)
	for i := 0; i < 20000; i++ {
		sign := 1
		if i%2 == 0 {
			sign = -1
		}
		delta := time.Duration(rand.Int63n(500)) * time.Millisecond
		dur := time.Second + time.Duration(sign*i)*time.Millisecond
		if dur < 0 {
			dur = 2 * time.Second
		}
		ds1[20000-1-i] = dur + delta
	}
	sort.Sort(ds1)
	rs1 := Summary{
		TestID:       time.Now().UTC().Format(time.RFC3339Nano),
		SuccessTotal: 10,
		FailureTotal: 10,
		Histogram: HistogramBuckets([]HistogramBucket{
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
		P50:   ds1.PickP50(),
		P90:   ds1.PickP90(),
		P99:   ds1.PickP99(),
		P999:  ds1.PickP999(),
		P9999: ds1.PickP9999(),
	}
	fmt.Println(rs1.JSON())
	fmt.Println(rs1.Table())

	println()
	ds2 := make(Durations, 20000)
	for i := 0; i < 20000; i++ {
		sign := 1
		if i%2 == 0 {
			sign = -1
		}
		delta := time.Duration(rand.Int63n(500)) * time.Millisecond
		dur := time.Second + time.Duration(sign*i)*time.Millisecond
		if dur < 0 {
			dur = 2 * time.Second
		}
		ds2[20000-1-i] = dur + delta
	}
	sort.Sort(ds2)
	rs2 := Summary{
		TestID:       time.Now().UTC().Format(time.RFC3339Nano),
		SuccessTotal: 10,
		FailureTotal: 10,
		Histogram: HistogramBuckets([]HistogramBucket{
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
		P50:   ds2.PickP50(),
		P90:   ds2.PickP90(),
		P99:   ds2.PickP99(),
		P999:  ds2.PickP999(),
		P9999: ds2.PickP9999(),
	}
	fmt.Println(rs2.JSON())
	fmt.Println(rs2.Table())

	println()
	c, err := CompareSummary(rs1, rs2)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(c.Table())
}

func TestMetricsHistogram(t *testing.T) {
	testMetric := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "test",
			Subsystem: "client",
			Name:      "request_latency_milliseconds",
			Help:      "Bucketed histogram of client-side request and response latency.",

			// lowest bucket start of upper bound 0.5 ms with factor 2
			// highest bucket start of 0.5 ms * 2^13 == 4.096 sec
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 14),
		})
	reg := prometheus.NewRegistry()
	if err := reg.Register(testMetric); err != nil {
		t.Skip(err)
	}
	defer reg.Unregister(testMetric)

	start := time.Now().Add(-10 * time.Minute)
	testMetric.Observe(float64(time.Since(start) / time.Millisecond))
	testMetric.Observe(float64(time.Since(start) / time.Millisecond))
	testMetric.Observe(float64(time.Since(time.Now().Add(-time.Millisecond)) / time.Millisecond))

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	var hs HistogramBuckets
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		if *mf.Name != "test_client_request_latency_milliseconds" {
			continue
		}
		hs, err = ParseHistogram("milliseconds", mf.Metric[0].Histogram)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected := HistogramBuckets([]HistogramBucket{
		{Scale: "milliseconds", LowerBound: 0, UpperBound: 0.5, Count: 0},
		{Scale: "milliseconds", LowerBound: 0.5, UpperBound: 1, Count: 1},
		{Scale: "milliseconds", LowerBound: 1, UpperBound: 2, Count: 0},
		{Scale: "milliseconds", LowerBound: 2, UpperBound: 4, Count: 0},
		{Scale: "milliseconds", LowerBound: 4, UpperBound: 8, Count: 0},
		{Scale: "milliseconds", LowerBound: 8, UpperBound: 16, Count: 0},
		{Scale: "milliseconds", LowerBound: 16, UpperBound: 32, Count: 0},
		{Scale: "milliseconds", LowerBound: 32, UpperBound: 64, Count: 0},
		{Scale: "milliseconds", LowerBound: 64, UpperBound: 128, Count: 0},
		{Scale: "milliseconds", LowerBound: 128, UpperBound: 256, Count: 0},
		{Scale: "milliseconds", LowerBound: 256, UpperBound: 512, Count: 0},
		{Scale: "milliseconds", LowerBound: 512, UpperBound: 1024, Count: 0},
		{Scale: "milliseconds", LowerBound: 1024, UpperBound: 2048, Count: 0},
		{Scale: "milliseconds", LowerBound: 2048, UpperBound: 4096, Count: 0},
		{Scale: "milliseconds", LowerBound: 4096, UpperBound: math.MaxFloat64, Count: 2},
	})
	for _, hv := range hs {
		fmt.Printf("%+v\n", hv)
	}
	if !reflect.DeepEqual(expected, hs) {
		t.Fatalf("expected %+v, got %+v", expected, hs)
	}

	fmt.Println(expected.Table())
}

func TestMergeHistogram(t *testing.T) {
	a := HistogramBuckets([]HistogramBucket{
		{Scale: "milliseconds", LowerBound: 0, UpperBound: 0.5, Count: 0},
		{Scale: "milliseconds", LowerBound: 0.5, UpperBound: 1, Count: 1},
		{Scale: "milliseconds", LowerBound: 1, UpperBound: 2, Count: 0},
		{Scale: "milliseconds", LowerBound: 2, UpperBound: 4, Count: 0},
		{Scale: "milliseconds", LowerBound: 4, UpperBound: 8, Count: 0},
		{Scale: "milliseconds", LowerBound: 8, UpperBound: 16, Count: 3},
		{Scale: "milliseconds", LowerBound: 16, UpperBound: 32, Count: 0},
		{Scale: "milliseconds", LowerBound: 32, UpperBound: 64, Count: 0},
		{Scale: "milliseconds", LowerBound: 64, UpperBound: 128, Count: 0},
		{Scale: "milliseconds", LowerBound: 128, UpperBound: 256, Count: 0},
		{Scale: "milliseconds", LowerBound: 256, UpperBound: 512, Count: 10},
		{Scale: "milliseconds", LowerBound: 512, UpperBound: 1024, Count: 0},
		{Scale: "milliseconds", LowerBound: 1024, UpperBound: 2048, Count: 0},
		{Scale: "milliseconds", LowerBound: 2048, UpperBound: 4096, Count: 0},
		{Scale: "milliseconds", LowerBound: 4096, UpperBound: math.MaxFloat64, Count: 2},
	})

	b := HistogramBuckets([]HistogramBucket{
		{Scale: "milliseconds", LowerBound: 0, UpperBound: 0.5, Count: 0},
		{Scale: "milliseconds", LowerBound: 0.5, UpperBound: 1, Count: 1},
		{Scale: "milliseconds", LowerBound: 1, UpperBound: 2, Count: 0},
		{Scale: "milliseconds", LowerBound: 2, UpperBound: 4, Count: 0},
		{Scale: "milliseconds", LowerBound: 4, UpperBound: 8, Count: 0},
		{Scale: "milliseconds", LowerBound: 8, UpperBound: 16, Count: 5},
		{Scale: "milliseconds", LowerBound: 16, UpperBound: 32, Count: 0},
		{Scale: "milliseconds", LowerBound: 32, UpperBound: 64, Count: 100},
		{Scale: "milliseconds", LowerBound: 64, UpperBound: 128, Count: 0},
		{Scale: "milliseconds", LowerBound: 128, UpperBound: 256, Count: 0},
		{Scale: "milliseconds", LowerBound: 256, UpperBound: 512, Count: 10},
		{Scale: "milliseconds", LowerBound: 512, UpperBound: 1024, Count: 0},
		{Scale: "milliseconds", LowerBound: 1024, UpperBound: 2048, Count: 0},
		{Scale: "milliseconds", LowerBound: 2048, UpperBound: 4096, Count: 0},
		{Scale: "milliseconds", LowerBound: 4096, UpperBound: math.MaxFloat64, Count: 2},
	})

	combined := HistogramBuckets([]HistogramBucket{
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
	})

	rs, err := MergeHistograms(a, b)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(combined, rs) {
		t.Fatalf("expected %+v, got %+v", combined, rs)
	}
}
