package client

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/aws/awstester/internal/eks/ingress/path"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// Client represents ingress load tester client.
type Client struct {
	lg *zap.Logger

	Endpoint string
	Routes   []string

	// ClientsN is the number of concurrent clients.
	ClientsN int
	wg       sync.WaitGroup

	// RequestsN is the number of total requests.
	RequestsN int64
	requestsN *atomic.Int64

	stopc chan struct{}
}

// New creates the client configuration.
func New(lg *zap.Logger, ep string, routesN int, clientsN int, requestsN int) (cli *Client, err error) {
	if routesN == 0 {
		return nil, errors.New("no routes found")
	}
	lg.Info("creating client",
		zap.String("endpoint", ep),
		zap.Int("routes", routesN),
		zap.Int("clients", clientsN),
		zap.Int("requests", requestsN),
	)
	routes := make([]string, routesN)
	for i := range routes {
		routes[i] = path.Create(i)
	}
	return &Client{
		lg:        lg,
		Endpoint:  ep,
		Routes:    routes,
		ClientsN:  clientsN,
		wg:        sync.WaitGroup{},
		RequestsN: int64(requestsN),
		requestsN: atomic.NewInt64(int64(requestsN)),
		stopc:     make(chan struct{}),
	}, nil
}

// TestResult contains test results.
type TestResult struct {
	mu       *sync.RWMutex
	Routes   int
	Clients  int
	Requests int64
	Success  int64
	Failure  int64
	Errors   []error
	QPS      float64
	Result   string
}

// Run runs load testing.
// It returns the result output and error count.
func (cli *Client) Run() (testResult TestResult) {
	now := time.Now().UTC()
	p := fmt.Sprintf("/%d", now.Nanosecond())
	mux := http.NewServeMux()
	mux.Handle(p, promhttp.Handler())
	ts := httptest.NewServer(mux)
	defer ts.Close()

	cli.lg.Info("started client load tester",
		zap.String("endpoint", cli.Endpoint),
		zap.Int("routes", len(cli.Routes)),
		zap.Int("clients", cli.ClientsN),
		zap.Int64("requests", cli.RequestsN),
	)

	testResult = TestResult{
		mu:       new(sync.RWMutex),
		Routes:   len(cli.Routes),
		Clients:  cli.ClientsN,
		Requests: cli.RequestsN,
	}

	cli.wg.Add(cli.ClientsN)
	for i := 0; i < cli.ClientsN; i++ {
		go func() {
			defer cli.wg.Done()
			for {
				select {
				case <-cli.stopc:
					return
				default:
				}

				left := cli.requestsN.Dec()
				if left < 0 {
					return
				}
				if left%100 == 0 {
					cli.lg.Info("request progress", zap.Int64("left", left), zap.Int64("total", cli.RequestsN))
				}

				route := cli.ChooseRoute()
				start := time.Now().UTC()

				// request
				rs, err := http.Get(cli.Endpoint + route)
				if err != nil {
					testResult.mu.Lock()
					testResult.Errors = append(testResult.Errors, err)
					testResult.mu.Unlock()
					cli.lg.Warn("request failed", zap.Error(err))
					promFailure.WithLabelValues(cli.Endpoint, route).Inc()
					if cli.requestsN.Dec() <= 0 {
						return
					}
					continue
				}
				if _, err = ioutil.ReadAll(rs.Body); err != nil {
					testResult.mu.Lock()
					testResult.Errors = append(testResult.Errors, err)
					testResult.mu.Unlock()
					cli.lg.Warn("request failed", zap.Error(err))
					promFailure.WithLabelValues(cli.Endpoint, route).Inc()
					if cli.requestsN.Dec() <= 0 {
						return
					}
					continue
				}
				if err = rs.Body.Close(); err != nil {
					testResult.mu.Lock()
					testResult.Errors = append(testResult.Errors, err)
					testResult.mu.Unlock()
					cli.lg.Warn("request failed", zap.Error(err))
					promFailure.WithLabelValues(cli.Endpoint, route).Inc()
					if cli.requestsN.Dec() <= 0 {
						return
					}
					continue
				}

				promLat.WithLabelValues(cli.Endpoint, route).Observe(time.Now().UTC().Sub(start).Seconds())
				promSuccess.WithLabelValues(cli.Endpoint, route).Inc()
			}
		}()
	}
	cli.wg.Wait()

	took := time.Now().UTC().Sub(now)
	cli.lg.Info("finished client load tester",
		zap.Int("clients", cli.ClientsN),
		zap.Int64("requests", cli.RequestsN),
	)

	// fetch metrics
	rs, err := http.Get(ts.URL + p)
	if err != nil {
		cli.lg.Warn("metrics fetch failed", zap.Error(err))
	}
	d, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		cli.lg.Warn("metrics fetch failed", zap.Error(err))
	}
	if err = rs.Body.Close(); err != nil {
		cli.lg.Warn("metrics fetch failed", zap.Error(err))
	}

	r := toResult(string(d))
	r.endpoint = cli.Endpoint
	r.routesN = len(cli.Routes)
	r.requestsN = cli.RequestsN
	r.clientsN = cli.ClientsN

	if r.successN > 0 {
		testResult.QPS = float64(r.successN) / took.Seconds()
	}

	testResult.Success = int64(r.successN)
	testResult.Failure = int64(r.failureN)
	testResult.Result = string(d) +
		r.String() +
		fmt.Sprintf("Took: %v\n", took) +
		fmt.Sprintf("QPS: %3.f successful requests per second\n", testResult.QPS) +
		fmt.Sprintf("Error count: %d\n", len(testResult.Errors))

	return testResult
}

// ChooseRoute chooses a random route.
func (cli *Client) ChooseRoute() string {
	rand.Seed(time.Now().UTC().UnixNano())
	return cli.Routes[rand.Intn(len(cli.Routes))]
}

// Stop stops load testing.
func (cli *Client) Stop() {
	cli.lg.Info("stopping")
	close(cli.stopc)
	cli.wg.Wait()
	cli.lg.Info("stopped")
}
