package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	humanize "github.com/dustin/go-humanize"
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

func TestGet(t *testing.T) {
	t.Skip()

	d, err := Read(zap.NewExample(), os.Stdout, "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/us-west-2/index.json")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("read", humanize.Bytes(uint64(len(d))))
}
