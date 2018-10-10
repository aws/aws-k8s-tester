// Package wrk implements wrk utilities.
package wrk

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"k8s.io/utils/exec"
)

// Config defines "wrk" command configuration.
// See https://github.com/wg/wrk for more.
type Config struct {
	Threads     int
	Connections int
	Duration    time.Duration
	Endpoint    string
}

// Run runs "wrk" commands.
func Run(cfg Config) (rs Result, err error) {
	ex := exec.New()
	if _, err = ex.LookPath("wrk"); err != nil {
		return Result{}, fmt.Errorf("wrk is not found (%v)", err)
	}

	args := []string{
		"--threads", fmt.Sprintf("%d", cfg.Threads),
		"--connections", fmt.Sprintf("%d", cfg.Connections),
		"--duration", fmt.Sprintf("%s", cfg.Duration),
		"--latency",
		cfg.Endpoint,
	}
	cmd := ex.CommandContext(context.Background(), "wrk", args...)
	var rbytes []byte
	rbytes, err = cmd.CombinedOutput()
	if err != nil {
		return Result{}, err
	}
	rs.Output = string(rbytes)

	return Parse(string(rbytes))
}

// Result defines "https://github.com/wg/wrk" command output
// with "--latency" flag. For example:
//
//	wrk \
//	  --threads 2 \
//	  --connections 200 \
//	  --duration 15s \
//	  --latency http://127.0.0.1
//
//	Running 15s test @ http://127.0.0.1
//	2 threads and 200 connections
//	Thread Stats   Avg      Stdev     Max   +/- Stdev
//		Latency   119.86ms  205.17ms   1.09s    85.26%
//		Req/Sec     9.42k     3.34k   14.13k    82.63%
//	Latency Distribution
//		50%    6.02ms
//		75%  169.67ms
//		90%  423.41ms
//		99%  885.55ms
//	  230174 requests in 15.01s, 8.83GB read
//	  Socket errors: connect 0, read 0, write 0, timeout 60
//	Requests/sec:  15335.21
//	Transfer/sec:    602.56MB
//
type Result struct {
	Output   string
	Duration time.Duration
	Endpoint string

	Threads        int64
	Connections    int64
	TotalTook      time.Duration
	RequestsPerSec float64

	TransferPerSec      string
	TransferPerSecBytes uint64

	TotalRequests      uint64
	TotalReadData      string
	TotalReadDataBytes uint64

	Latency50Pct time.Duration
	Latency75Pct time.Duration
	Latency90Pct time.Duration
	Latency99Pct time.Duration

	LatencyAvg          time.Duration
	RequestsPerSecAvg   float64
	LatencyStdev        time.Duration
	RequestsPerSecStdev float64
	LatencyMax          time.Duration
	RequestsPerSecMax   float64

	ErrorsConnect int64
	ErrorsRead    int64
	ErrorsWrite   int64
	ErrorsTimeout int64
}

var header = []string{
	"threads",
	"connections",
	"total-took",
	"requests-per-sec",

	"transfer-per-sec",
	"transfer-per-sec-bytes",

	"total-requests",
	"total-read-data",
	"total-read-data-bytes",

	"latency-50pct",
	"latency-75pct",
	"latency-90pct",
	"latency-99pct",

	"latency-avg",
	"requests-per-sec-avg",
	"latency-stdev",
	"requests-per-sec-stdev",
	"latency-max",
	"requests-per-sec-max",

	"errors-connect",
	"errors-read",
	"errors-write",
	"errors-timeout",
}

// Parse parses "wrk" command output.
func Parse(s string) (rs Result, err error) {
	rs.Output = s

	prevThreadStatsHeader := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {

		case strings.Contains(line, "Running") && strings.Contains(line, "test @"):
			// Running 15s test @ http://127.0.0.1
			old := line
			line = strings.Replace(line, "Running ", " ", -1)
			line = strings.Replace(line, "test @", " ", -1)
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return Result{}, fmt.Errorf("expected 2 fields from %q", old)
			}
			rs.Duration, _ = time.ParseDuration(fields[0])
			rs.Endpoint = strings.TrimSpace(fields[1])

		case strings.Contains(line, " threads and ") &&
			strings.Contains(line, " connections"):
			old := line
			line = strings.Replace(line, " threads and ", " ", -1)
			line = strings.Replace(line, " connections", " ", -1)
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return Result{}, fmt.Errorf("expected 2 numbers from %q", old)
			}
			rs.Threads, _ = strconv.ParseInt(fields[0], 10, 64)
			rs.Connections, _ = strconv.ParseInt(fields[1], 10, 64)

		case strings.HasPrefix(line, "Thread Stats"):
			prevThreadStatsHeader = true
			continue

		case strings.HasPrefix(line, "Latency Distribution"):
			continue

		case prevThreadStatsHeader &&
			strings.HasPrefix(line, "Latency"):
			old := line
			line = strings.Replace(line, "Latency", "", -1)
			fields := strings.Fields(line)
			if len(fields) != 4 {
				return Result{}, fmt.Errorf("expected 4 numbers from %q", old)
			}
			rs.LatencyAvg, _ = time.ParseDuration(fields[0])
			rs.LatencyStdev, _ = time.ParseDuration(fields[1])
			rs.LatencyMax, _ = time.ParseDuration(fields[2])
			prevThreadStatsHeader = false

		case strings.HasPrefix(line, "Req/Sec"):
			old := line
			line = strings.Replace(line, "Req/Sec", "", -1)
			fields := strings.Fields(line)
			if len(fields) != 4 {
				return Result{}, fmt.Errorf("expected 4 numbers from %q", old)
			}
			rs.RequestsPerSecAvg = parseWithK(fields[0])
			rs.RequestsPerSecStdev = parseWithK(fields[1])
			rs.RequestsPerSecMax = parseWithK(fields[2])

		case strings.HasPrefix(line, "50%"):
			line = strings.TrimSpace(strings.Replace(line, "50%", "", -1))
			rs.Latency50Pct, _ = time.ParseDuration(line)

		case strings.HasPrefix(line, "75%"):
			line = strings.TrimSpace(strings.Replace(line, "75%", "", -1))
			rs.Latency75Pct, _ = time.ParseDuration(line)

		case strings.HasPrefix(line, "90%"):
			line = strings.TrimSpace(strings.Replace(line, "90%", "", -1))
			rs.Latency90Pct, _ = time.ParseDuration(line)

		case strings.HasPrefix(line, "99%"):
			line = strings.TrimSpace(strings.Replace(line, "99%", "", -1))
			rs.Latency99Pct, _ = time.ParseDuration(line)

		case strings.Contains(line, " requests in ") && strings.Contains(line, " read"):
			// 148205 requests in 10.03s, 5.69GB read
			old := line
			line = strings.Replace(line, " requests in ", " ", -1)
			line = strings.Replace(line, ", ", " ", -1)
			line = strings.Replace(line, " read", "", -1)
			fields := strings.Fields(line)
			if len(fields) != 3 {
				return Result{}, fmt.Errorf("expected 3 numbers from %q", old)
			}
			rs.TotalRequests, _ = strconv.ParseUint(fields[0], 10, 64)
			rs.TotalTook, _ = time.ParseDuration(fields[1])
			rs.TotalReadData = fields[2]
			rs.TotalReadDataBytes, _ = humanize.ParseBytes(rs.TotalReadData)

		case strings.HasPrefix(line, "Socket errors:"):
			old := line
			line = strings.Replace(line, "Socket errors:", "", -1)
			line = strings.TrimSpace(line)
			fields := strings.Split(line, ", ")
			if len(fields) != 4 {
				return Result{}, fmt.Errorf("expected 4 fields from %q", old)
			}
			rs.ErrorsConnect, _ = strconv.ParseInt(strings.Fields(fields[0])[1], 10, 64)
			rs.ErrorsRead, _ = strconv.ParseInt(strings.Fields(fields[1])[1], 10, 64)
			rs.ErrorsWrite, _ = strconv.ParseInt(strings.Fields(fields[2])[1], 10, 64)
			rs.ErrorsTimeout, _ = strconv.ParseInt(strings.Fields(fields[3])[1], 10, 64)

		case strings.HasPrefix(line, "Requests/sec:"):
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return Result{}, fmt.Errorf("expected 2 fields from %q", line)
			}
			sv := fields[1]
			rs.RequestsPerSec, _ = strconv.ParseFloat(sv, 64)

		case strings.HasPrefix(line, "Transfer/sec:"):
			line = strings.Replace(line, "Transfer/sec:", "", -1)
			line = strings.TrimSpace(line)
			rs.TransferPerSec = line
			rs.TransferPerSecBytes, _ = humanize.ParseBytes(rs.TransferPerSec)
		}
	}
	return rs, err
}

// Combine combines multiple results into one.
func Combine(rss ...Result) (rs Result) {
	for i, v := range rss {
		if i == 0 {
			rs = v
			continue
		}

		rs.RequestsPerSec += v.RequestsPerSec
		rs.RequestsPerSec /= 2.0

		rs.TransferPerSecBytes += v.TransferPerSecBytes
		rs.TransferPerSecBytes /= 2.0
		rs.TransferPerSec = humanize.Bytes(rs.TransferPerSecBytes)

		rs.TotalRequests += v.TotalRequests
		rs.TotalReadDataBytes += v.TotalReadDataBytes
		rs.TotalReadDataBytes /= 2.0
		rs.TotalReadData = humanize.Bytes(rs.TotalReadDataBytes)

		rs.LatencyAvg += v.LatencyAvg
		rs.LatencyAvg /= 2
		rs.RequestsPerSecAvg += v.RequestsPerSecAvg
		rs.RequestsPerSecAvg /= 2.0

		rs.LatencyStdev += v.LatencyStdev
		rs.LatencyStdev /= 2
		rs.RequestsPerSecStdev += v.RequestsPerSecStdev
		rs.RequestsPerSecStdev /= 2.0

		rs.LatencyMax += v.LatencyMax
		rs.LatencyMax /= 2
		rs.RequestsPerSecMax += v.RequestsPerSecMax
		rs.RequestsPerSecMax /= 2.0

		rs.ErrorsConnect += v.ErrorsConnect
		rs.ErrorsRead += v.ErrorsRead
		rs.ErrorsWrite += v.ErrorsWrite
		rs.ErrorsTimeout += v.ErrorsTimeout
	}

	rs.Latency50Pct = 0
	rs.Latency75Pct = 0
	rs.Latency90Pct = 0
	rs.Latency99Pct = 0

	return rs
}

// ToCSV converts a list of Result to a CSV file.
func ToCSV(output string, rss ...Result) error {
	rows := make([][]string, 0, len(rss))
	for _, v := range rss {
		rows = append(rows, []string{
			fmt.Sprintf("%d", v.Threads),        // "threads"
			fmt.Sprintf("%d", v.Connections),    // "connections"
			fmt.Sprintf("%v", v.TotalTook),      // "took"
			fmt.Sprintf("%f", v.RequestsPerSec), // "requests-per-sec"

			v.TransferPerSec,                         // "transfer-per-sec"
			fmt.Sprintf("%d", v.TransferPerSecBytes), // "transfer-per-sec-bytes"

			fmt.Sprintf("%d", v.TotalRequests),      // "total-requests"
			v.TotalReadData,                         // "total-read-data"
			fmt.Sprintf("%d", v.TotalReadDataBytes), // "total-read-data-bytes"

			fmt.Sprintf("%v", v.Latency50Pct), // "latency-50pct"
			fmt.Sprintf("%v", v.Latency75Pct), // "latency-75pct"
			fmt.Sprintf("%v", v.Latency90Pct), // "latency-90pct"
			fmt.Sprintf("%v", v.Latency99Pct), // "latency-99pct"

			fmt.Sprintf("%v", v.LatencyAvg),          // "latency-avg"
			fmt.Sprintf("%f", v.RequestsPerSecAvg),   // "requests-per-sec-avg"
			fmt.Sprintf("%v", v.LatencyStdev),        // "latency-stdev"
			fmt.Sprintf("%f", v.RequestsPerSecStdev), // "requests-per-sec-stdev"
			fmt.Sprintf("%v", v.LatencyMax),          // "latency-max"
			fmt.Sprintf("%f", v.RequestsPerSecMax),   // "requests-per-sec-max"

			fmt.Sprintf("%d", v.ErrorsConnect), // "errors-connect"
			fmt.Sprintf("%d", v.ErrorsRead),    // "errors-read"
			fmt.Sprintf("%d", v.ErrorsWrite),   // "errors-write"
			fmt.Sprintf("%d", v.ErrorsTimeout), // "errors-timeout"
		})
	}
	return toCSV(output, header, rows)
}

// parses "9.2k" to 9200, "9.65k" to 9650
func parseWithK(s string) float64 {
	fv, err := strconv.ParseFloat(s, 64)
	if err != nil &&
		strings.Contains(s, ".") &&
		strings.HasSuffix(s, "k") {
		pv, _ := strconv.ParseFloat(strings.Replace(s, "k", "", -1), 64)
		return pv * 1000
	}
	return fv
}

func toCSV(output string, header []string, rows [][]string) error {
	f, err := os.OpenFile(output, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		f, err = os.Create(output)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	wr := csv.NewWriter(f)
	if err := wr.Write(header); err != nil {
		return err
	}
	if err := wr.WriteAll(rows); err != nil {
		return err
	}
	wr.Flush()
	return wr.Error()
}
