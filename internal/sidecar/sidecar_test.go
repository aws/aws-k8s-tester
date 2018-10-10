package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestNewMux(t *testing.T) {
	p := "/sidecar"
	mux, err := NewMux(context.Background(), zap.NewExample(), p)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	req := Request{
		TargetURL: "https://httpbin.org/get",
		Method:    http.MethodGet,
	}
	sd, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	rs, err := http.Post(ts.URL+p, "application/json", bytes.NewReader(sd))
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
	fmt.Println(string(d))
}
