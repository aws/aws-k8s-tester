package server

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/awstester/internal/eks/alb/ingress/path"
	"go.uber.org/zap"
)

func TestNewMux(t *testing.T) {
	mux, err := NewMux(context.Background(), zap.NewExample(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// send request
	rs, err := http.Get(ts.URL + path.Create(0))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = rs.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d, []byte("0000000000")) {
		t.Fatalf("expected %q, got %q", "0000000000", string(d))
	}

	// check response metrics
	rs, err = http.Get(ts.URL + path.PathMetrics)
	if err != nil {
		t.Fatal(err)
	}
	d, err = ioutil.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err = rs.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(d), "ingress_test_server_count_recv") {
		t.Fatalf("unexpected metrics output %q", string(d))
	}
	if !strings.Contains(string(d), "ingress_test_server_latency_sent") {
		t.Fatalf("unexpected metrics output %q", string(d))
	}
}
