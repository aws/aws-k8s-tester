package status

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestNewMux(t *testing.T) {
	// to limit GitHub API calls
	// or excessive Kubernetes source code downloads
	t.Skip()

	mux := NewMux(context.Background(), zap.NewExample())
	ts := httptest.NewServer(mux)
	defer ts.Close()

	_, err := http.Get(ts.URL + Path)
	if err != nil {
		t.Fatal(err)
	}
	// expect rate limit
	_, err = http.Get(ts.URL + Path)
	if err != nil {
		t.Fatal(err)
	}
	var rs *http.Response
	rs, err = http.Get(ts.URL + Path)
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	body, err = ioutil.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "rate limit exceeded ") {
		t.Fatal("'rate limit exceeded' expected")
	}
	fmt.Println(string(body))

	rs, err = http.Get(ts.URL + "/not-exists")
	if err != nil {
		t.Fatal(err)
	}
	if rs.StatusCode != http.StatusNotFound {
		t.Errorf("expected status code %v, got %v", http.StatusNotFound, rs.StatusCode)
	}
}
