// Package latency defines latency utilities.
package latency

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/olekukonko/tablewriter"
	dto "github.com/prometheus/client_model/go"
)

// SummaryCompare compares two "Summary".
// Delta is computed with "A" as "before" and with "B" as "after".
type SummaryCompare struct {
	A Summary `json:"a" read-only:"true"`
	B Summary `json:"b" read-only:"true"`

	P50DeltaPercent   float64 `json:"latency-p50-delta-percent" read-only:"true"`
	P90DeltaPercent   float64 `json:"latency-p90-delta-percent" read-only:"true"`
	P99DeltaPercent   float64 `json:"latency-p99-delta-percent" read-only:"true"`
	P999DeltaPercent  float64 `json:"latency-p99.9-delta-percent" read-only:"true"`
	P9999DeltaPercent float64 `json:"latency-p99.99-delta-percent" read-only:"true"`
}

func (c SummaryCompare) JSON() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func (c SummaryCompare) Table() string {
	buf := bytes.NewBuffer(nil)
	tb := tablewriter.NewWriter(buf)
	tb.SetAutoWrapText(false)
	tb.SetColWidth(1500)
	tb.SetCenterSeparator("*")
	tb.SetAlignment(tablewriter.ALIGN_CENTER)
	tb.SetCaption(true, "(% delta from 'A' to 'B')")
	tb.SetHeader([]string{"Percentile", fmt.Sprintf("A %q", c.A.TestID), fmt.Sprintf("B %q", c.B.TestID), "Delta"})

	tb.Append([]string{"50-pct Latency", c.A.P50.String(), c.B.P50.String(), toPercent(c.P50DeltaPercent)})
	tb.Append([]string{"90-pct Latency", c.A.P90.String(), c.B.P90.String(), toPercent(c.P90DeltaPercent)})
	tb.Append([]string{"99-pct Latency", c.A.P99.String(), c.B.P99.String(), toPercent(c.P99DeltaPercent)})
	tb.Append([]string{"99.9-pct Latency", c.A.P999.String(), c.B.P999.String(), toPercent(c.P999DeltaPercent)})
	tb.Append([]string{"99.99-pct Latency", c.A.P9999.String(), c.B.P9999.String(), toPercent(c.P9999DeltaPercent)})

	tb.Render()
	return buf.String()
}

func toPercent(f float64) string {
	sign := "+"
	if f < 0.0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.3f %%", sign, f)
}

// CompareSummary compares two "Summary".
func CompareSummary(a Summary, b Summary) (c SummaryCompare, err error) {
	if len(a.Histogram) != len(b.Histogram) {
		return SummaryCompare{}, fmt.Errorf("len(a.Histogram) %d != len(b.Histogram) %d", len(a.Histogram), len(b.Histogram))
	}

	c = SummaryCompare{
		A: a,
		B: b,
	}

	// e.g. "A" 100, "B" 50 == -50%
	// e.g. "A" 50, "B" 100 == 100%
	deltaP50 := float64(b.P50) - float64(a.P50)
	deltaP50 /= float64(a.P50)
	deltaP50 *= 100.0
	deltaP50 = convertInvalid(deltaP50)

	deltaP90 := float64(b.P90) - float64(a.P90)
	deltaP90 /= float64(a.P90)
	deltaP90 *= 100.0
	deltaP90 = convertInvalid(deltaP90)

	deltaP99 := float64(b.P99) - float64(a.P99)
	deltaP99 /= float64(a.P99)
	deltaP99 *= 100.0
	deltaP99 = convertInvalid(deltaP99)

	deltaP999 := float64(b.P999) - float64(a.P999)
	deltaP999 /= float64(a.P999)
	deltaP999 *= 100.0
	deltaP999 = convertInvalid(deltaP999)

	deltaP9999 := float64(b.P9999) - float64(a.P9999)
	deltaP9999 /= float64(a.P9999)
	deltaP9999 *= 100.0
	deltaP9999 = convertInvalid(deltaP9999)

	c.P50DeltaPercent = deltaP50
	c.P90DeltaPercent = deltaP90
	c.P99DeltaPercent = deltaP99
	c.P999DeltaPercent = deltaP999
	c.P9999DeltaPercent = deltaP9999

	return c, nil
}

func convertInvalid(f float64) (v float64) {
	v = f
	if math.IsNaN(f) {
		v = 0
	}
	if math.IsInf(f, 1) {
		v = math.MaxFloat64
	}
	if math.IsInf(f, -1) {
		v = -math.MaxFloat64
	}
	return v
}

// Summary represents request results.
type Summary struct {
	// TestID is the test ID.
	TestID string `json:"test_id" read-only:"true"`

	// SuccessTotal is the number of successful client requests.
	SuccessTotal float64 `json:"success_total" read-only:"true"`
	// FailureTotal is the number of failed client requests.
	FailureTotal float64 `json:"failure_total" read-only:"true"`
	// Histogram is the client requests latency histogram.
	Histogram HistogramBuckets `json:"histogram" read-only:"true"`

	// P50 is the 50-percentile latency.
	P50 time.Duration `json:"p50" read-only:"true"`
	// P90 is the 90-percentile latency.
	P90 time.Duration `json:"p90" read-only:"true"`
	// P99 is the 99-percentile latency.
	P99 time.Duration `json:"p99" read-only:"true"`
	// P999 is the 99.9-percentile latency.
	P999 time.Duration `json:"p99.9" read-only:"true"`
	// P9999 is the 99.99-percentile latency.
	P9999 time.Duration `json:"p99.99" read-only:"true"`
}

func (rs Summary) JSON() string {
	b, _ := json.Marshal(rs)
	return string(b)
}

func (rs Summary) Table() string {
	return fmt.Sprintf(`
TEST ID: %q

        TOTAL: %.2f
SUCCESS TOTAL: %.2f
FAILURE TOTAL: %.2f

`,
		rs.TestID,
		rs.SuccessTotal+rs.FailureTotal,
		rs.SuccessTotal,
		rs.FailureTotal,
	) +
		rs.Histogram.Table() +
		fmt.Sprintf(`
   50-percentile Latency: %s
   90-percentile Latency: %s
   99-percentile Latency: %s
 99.9-percentile Latency: %s
99.99-percentile Latency: %s

`,
			rs.P50,
			rs.P90,
			rs.P99,
			rs.P999,
			rs.P9999,
		)
}

// DurationWithLabel is the duration with label.
// ref. https://en.wikipedia.org/wiki/Kolmogorov%E2%80%93Smirnov_test
type DurationWithLabel struct {
	time.Duration
	Label string
}

type DurationWithLabels []DurationWithLabel

func (ds DurationWithLabels) Len() int           { return len(ds) }
func (ds DurationWithLabels) Less(i, j int) bool { return ds[i].Duration < ds[j].Duration }
func (ds DurationWithLabels) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }

// PickP50 returns the latency assuming durations are already sorted.
func (ds DurationWithLabels) PickP50() DurationWithLabel {
	n := len(ds)
	if n == 0 {
		return DurationWithLabel{}
	}
	if n == 1 {
		return ds[0]
	}

	idx := n / 2
	return ds[idx]
}

// PickP90 returns the latency assuming durations are already sorted.
func (ds DurationWithLabels) PickP90() DurationWithLabel {
	n := len(ds)
	if n == 0 {
		return DurationWithLabel{}
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

// PickP99 returns the latency assuming durations are already sorted.
func (ds DurationWithLabels) PickP99() DurationWithLabel {
	n := len(ds)
	if n == 0 {
		return DurationWithLabel{}
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

// PickP999 returns the latency assuming durations are already sorted.
func (ds DurationWithLabels) PickP999() DurationWithLabel {
	n := len(ds)
	if n == 0 {
		return DurationWithLabel{}
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

// PickP9999 returns the latency assuming durations are already sorted.
func (ds DurationWithLabels) PickP9999() DurationWithLabel {
	n := len(ds)
	if n == 0 {
		return DurationWithLabel{}
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

func (ds DurationWithLabels) CSV(path string) error {
	csvFile, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		csvFile, err = os.Create(path)
		if err != nil {
			return err
		}
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	csvWriter.Write([]string{"label", "duration-ms"})

	rows := make([][]string, len(ds))
	for idx := range ds {
		rows[idx] = []string{ds[idx].Label, fmt.Sprintf("%d", ds[idx].Milliseconds())}
	}
	return csvWriter.WriteAll(rows)
}

// LabelDurations labels durations.
func LabelDurations(ds1 Durations, label string) (ds2 DurationWithLabels) {
	ds2 = make(DurationWithLabels, len(ds1), len(ds1))
	for idx := range ds1 {
		ds2[idx] = DurationWithLabel{
			Duration: ds1[idx],
			Label:    label,
		}
	}
	return ds2
}

type Durations []time.Duration

func (ds Durations) Len() int           { return len(ds) }
func (ds Durations) Less(i, j int) bool { return ds[i] < ds[j] }
func (ds Durations) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }

// PickP50 returns the latency assuming durations are already sorted.
func (ds Durations) PickP50() time.Duration {
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

// PickP90 returns the latency assuming durations are already sorted.
func (ds Durations) PickP90() time.Duration {
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

// PickP99 returns the latency assuming durations are already sorted.
func (ds Durations) PickP99() time.Duration {
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

// PickP999 returns the latency assuming durations are already sorted.
func (ds Durations) PickP999() time.Duration {
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

// PickP9999 returns the latency assuming durations are already sorted.
func (ds Durations) PickP9999() time.Duration {
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

// Table converts "HistogramBuckets" to table.
func (buckets HistogramBuckets) Table() string {
	if len(buckets) == 0 {
		return ""
	}
	buf := bytes.NewBuffer(nil)
	tb := tablewriter.NewWriter(buf)
	tb.SetAutoWrapText(false)
	tb.SetColWidth(1500)
	tb.SetCenterSeparator("*")
	tb.SetAlignment(tablewriter.ALIGN_CENTER)
	tb.SetCaption(true, fmt.Sprintf("	(%q scale)", buckets[0].Scale))
	tb.SetHeader([]string{"lower bound", "upper bound", "count"})
	for _, v := range buckets {
		lo := fmt.Sprintf("%f", v.LowerBound)
		if v.Scale == "milliseconds" {
			lo = fmt.Sprintf("%.3f", v.LowerBound)
		}
		hi := fmt.Sprintf("%f", v.UpperBound)
		if v.Scale == "milliseconds" {
			hi = fmt.Sprintf("%.3f", v.UpperBound)
		}
		if v.UpperBound == math.MaxFloat64 {
			hi = "math.MaxFloat64"
		}
		tb.Append([]string{lo, hi, fmt.Sprintf("%d", v.Count)})
	}
	tb.Render()
	return buf.String()
}
