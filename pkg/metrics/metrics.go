// Package metrics defines metrics utilities.
package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	dto "github.com/prometheus/client_model/go"
)

// HistogramBucket represents metrics latency bucket.
type HistogramBucket struct {
	Scale      string  `json:"scale"`
	LowerBound float64 `json:"lower-bound"`
	UpperBound float64 `json:"upper-bound"`
	Count      uint64  `json:"count"`
}

func (bucket HistogramBucket) String() string {
	b, _ := json.Marshal(bucket)
	return string(b)
}

type HistogramBuckets []HistogramBucket

func (buckets HistogramBuckets) Len() int { return len(buckets) }

func (buckets HistogramBuckets) Less(i, j int) bool {
	return buckets[i].LowerBound < buckets[j].LowerBound
}

func (buckets HistogramBuckets) Swap(i, j int) {
	t := buckets[i]
	buckets[i] = buckets[j]
	buckets[j] = t
}

// ParseHistogram parses Prometheus histogram.
func ParseHistogram(scale string, histo *dto.Histogram) (buckets HistogramBuckets, err error) {
	if histo == nil {
		return nil, errors.New("nil Histogram")
	}

	total := *histo.SampleCount
	n := len(histo.Bucket)

	buckets = make(HistogramBuckets, n+1)
	buckets[n] = HistogramBucket{
		Scale:      scale,
		UpperBound: math.MaxFloat64,
		Count:      total,
	}
	for idx, bv := range histo.Bucket {
		buckets[idx] = HistogramBucket{
			Scale:      scale,
			UpperBound: *bv.UpperBound,
			Count:      *bv.CumulativeCount,
		}
	}
	for idx := n; idx > 0; idx-- { // start from last, end at second to last
		// convert cumulative count to per-bucket count
		buckets[idx].Count = buckets[idx].Count - buckets[idx-1].Count
		// use previous bucket upper bound as lower bound
		buckets[idx].LowerBound = buckets[idx-1].UpperBound
	}

	sort.Sort(HistogramBuckets(buckets))
	return buckets, nil
}

// MergeHistograms merges two histograms.
func MergeHistograms(a HistogramBuckets, b HistogramBuckets) (HistogramBuckets, error) {
	counts := make(map[HistogramBucket]uint64)
	for _, cur := range a {
		key := HistogramBucket{
			Scale:      cur.Scale,
			LowerBound: cur.LowerBound,
			UpperBound: cur.UpperBound,
		}
		counts[key] = cur.Count
	}
	for _, cur := range b {
		key := HistogramBucket{
			Scale:      cur.Scale,
			LowerBound: cur.LowerBound,
			UpperBound: cur.UpperBound,
		}
		prevCount, ok := counts[key]
		if !ok {
			return nil, fmt.Errorf("[%f, %f] not found in previous counts", cur.LowerBound, cur.UpperBound)
		}
		counts[key] = prevCount + cur.Count
	}
	hs := make(HistogramBuckets, 0, len(counts))
	for k, v := range counts {
		hs = append(hs, HistogramBucket{
			Scale:      k.Scale,
			LowerBound: k.LowerBound,
			UpperBound: k.UpperBound,
			Count:      v,
		})
	}
	sort.Sort(HistogramBuckets(hs))
	return hs, nil
}
