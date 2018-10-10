package client

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	promSuccess = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "ingress_client",
		Name:      "count_success",
		Help:      "total number of successful client requests",
	},
		[]string{"Endpoint", "Route"},
	)
	promFailure = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "ingress_client",
		Name:      "count_failure",
		Help:      "total number of failed client requests",
	},
		[]string{"Endpoint", "Route"},
	)
	promLat = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "ingress_client",
		Name:      "latency",
		Help:      "total latency distributions of requests, in seconds",

		// lowest bucket start of upper bound 0.0001 sec (0.1 ms) with factor 2
		// highest bucket start of 0.0001 sec * 2^17 == 13.1072 sec
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 18),
	},
		[]string{"Endpoint", "Route"},
	)
)

func init() {
	prometheus.MustRegister(promSuccess)
	prometheus.MustRegister(promFailure)
	prometheus.MustRegister(promLat)
}

type countMetrics struct {
	Endpoint string
	Route    string
	Count    string
	count    int64
}

// parse 'ingress_client_count_success{Endpoint="URL",Route="/ingress-test-00028"} 307'
// parse 'ingress_client_count_failure{Endpoint="URL",Route="/ingress-test-00028"} 307'
func parseCountMetrics(line string) (cm countMetrics) {
	line = strings.Replace(line, "ingress_client_count_success", "", -1)
	line = strings.Replace(line, "ingress_client_count_failure", "", -1)
	line = strings.Replace(line, `{Endpoint="`, "", -1)
	cm.Endpoint, line = strings.Split(line, `",Route="`)[0], strings.Split(line, `",Route="`)[1]
	cm.Route, cm.Count = strings.Split(line, `"} `)[0], strings.Split(line, `"} `)[1]
	var err error
	cm.count, err = strconv.ParseInt(cm.Count, 10, 64)
	if err != nil {
		panic(err)
	}
	return cm
}

type histogramMetrics struct {
	Route       string
	To          string
	le          string
	lessEqualMs float64
	count       int64
}

// parse 'ingress_client_latency_bucket{Route="URL",To="/ingress-test-00001",le="0.0001"} 0'
func parseHistogramMetrics(line string) (hm histogramMetrics) {
	line = strings.Replace(line, "ingress_client_latency_bucket", "", -1)
	line = strings.Replace(line, `{Endpoint="`, "", -1)
	hm.Route, line = strings.Split(line, `",Route="`)[0], strings.Split(line, `",Route="`)[1]
	hm.To, line = strings.Split(line, `",le="`)[0], strings.Split(line, `",le="`)[1]
	hm.le, line = strings.Split(line, `"} `)[0], strings.Split(line, `"} `)[1]
	var err error
	hm.lessEqualMs, err = strconv.ParseFloat(hm.le, 64)
	if err != nil {
		panic(err)
	}
	hm.lessEqualMs *= 1000 // second to millisecond
	hm.count, err = strconv.ParseInt(line, 10, 64)
	if err != nil {
		panic(err)
	}
	return hm
}

type result struct {
	endpoint string

	requestsN int64
	successN  int64
	failureN  int64

	clientsN int

	routesN int

	latencies []latency
}

type latency struct {
	MsFrom    float64
	msFrom    string
	MsTo      float64
	msTo      string
	RequestsN int64
}

type latencies []latency

func (ls latencies) Len() int      { return len(ls) }
func (ls latencies) Swap(i, j int) { ls[i], ls[j] = ls[j], ls[i] }
func (ls latencies) Less(i, j int) bool {
	return ls[i].MsFrom < ls[j].MsFrom
}

func toResult(s string) (rs result) {
	hss := make([]histogramMetrics, 0)
	prev := histogramMetrics{}
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "# "):
			continue

		case strings.HasPrefix(line, "ingress_client_count_success"):
			cm := parseCountMetrics(line)
			rs.successN += cm.count
		case strings.HasPrefix(line, "ingress_client_count_failure"):
			cm := parseCountMetrics(line)
			rs.failureN += cm.count

		case strings.HasPrefix(line, "ingress_client_latency_bucket"):
			hm := parseHistogramMetrics(line)
			if prev.Route != "" && prev.Route == hm.Route {
				// compute delta
				copied := prev
				prev = hm
				hm.count -= copied.count
			} else {
				prev = hm
			}
			hss = append(hss, hm)
		}
	}

	// find all ranges
	lessEqualMsToMsFrom := make(map[float64]float64)
	prev = histogramMetrics{}
	prevMs := float64(-1)
	for _, hv := range hss {
		if prev.Route == "" && prevMs == float64(-1) {
			prevMs = hv.lessEqualMs
		} else if prev.Route != hv.Route {
			// first iteration has completed
			break
		}
		lessEqualMsToMsFrom[hv.lessEqualMs] = prevMs
		prev, prevMs = hv, hv.lessEqualMs
	}

	// aggregate by ranges
	r2cnt := make(map[latency]latency)
	for _, hv := range hss {
		k := latency{
			msFrom: fmt.Sprintf("%.5f", lessEqualMsToMsFrom[hv.lessEqualMs]),
			msTo:   fmt.Sprintf("%.5f", hv.lessEqualMs),
		}
		if v, ok := r2cnt[k]; ok {
			v.RequestsN += hv.count
			r2cnt[k] = v
		} else {
			r2cnt[k] = latency{
				MsFrom:    lessEqualMsToMsFrom[hv.lessEqualMs],
				MsTo:      hv.lessEqualMs,
				RequestsN: hv.count,
			}
		}
	}
	lss := make([]latency, 0, len(r2cnt))
	for _, v := range r2cnt {
		if v.MsFrom == math.Inf(1) {
			continue
		}
		lss = append(lss, v)
	}
	rs.latencies = lss
	sort.Sort(latencies(rs.latencies))
	return rs
}

func (rs result) String() (s string) {
	s += "\n"
	s += fmt.Sprintf("Endpoint: %q\n", rs.endpoint)
	s += "\n"
	s += fmt.Sprintf("Success: %d\n", rs.successN)
	s += fmt.Sprintf("   Fail: %d\n", rs.failureN)
	s += "\n"
	s += fmt.Sprintf("Total Concurrent Clients: %d\n", rs.clientsN)
	s += "\n"
	s += fmt.Sprintf("Total ALB Targets: %d\n", rs.routesN)
	s += "\n"
	s += "Latency Distribution:\n"
	for i, lat := range rs.latencies {
		if i == len(rs.latencies)-1 {
			s += fmt.Sprintf("%.1f ~ +Inf ms: %d requests\n", lat.MsFrom, lat.RequestsN)
		} else {
			s += fmt.Sprintf("%.1f ~ %.1f ms: %d requests\n", lat.MsFrom, lat.MsTo, lat.RequestsN)
		}
	}
	s += "\n"
	return s
}
