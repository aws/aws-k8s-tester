package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestCheckGet(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("OK"))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	if !CheckGet(zap.NewExample(), ts.URL+"/hello", "OK", 10, time.Second, nil) {
		t.Fatal("unexpected response")
	}
}
