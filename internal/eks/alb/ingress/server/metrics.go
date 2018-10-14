package server

import "github.com/prometheus/client_golang/prometheus"

var (
	promRecv = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "ingress_test_server",
		Name:      "count_recv",
		Help:      "total number of received requests",
	},
		[]string{"From", "Method", "Path"},
	)
	promSent = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "ingress_test_server",
		Name:      "latency_sent",
		Help:      "total latency distributions of sent responses",

		// lowest bucket start of upper bound 0.0001 sec (0.1 ms) with factor 2
		// highest bucket start of 0.0001 sec * 2^17 == 13.1072 sec
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 18),
	},
		[]string{"Path", "To"},
	)
)

func init() {
	prometheus.MustRegister(promRecv)
	prometheus.MustRegister(promSent)
}
