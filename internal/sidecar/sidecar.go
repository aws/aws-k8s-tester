// Package sidecar implements simple pod inspector.
package sidecar

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/aws/awstester/pkg/ctxhandler"

	"go.uber.org/zap"
)

// NewMux returns a new HTTP request multiplexer with registered handlers.
// Specify the path to serve sidecar.
func NewMux(ctx context.Context, lg *zap.Logger, p string) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.Handle(p, &ctxhandler.ContextAdapter{
		Logger:  lg,
		Ctx:     ctx,
		Handler: ctxhandler.ContextHandlerFunc(handler),
	})
	return mux, nil
}

// Request defines sidecar request.
type Request struct {
	TargetURL string `json:"target-url"`
	Method    string `json:"method"`
}

func handler(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	switch req.Method {
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("sidecar is healthy"))

	case http.MethodPut,
		http.MethodPost:
		var creq Request
		if err := json.NewDecoder(req.Body).Decode(&creq); err != nil {
			return err
		}
		defer req.Body.Close()

		switch creq.Method {
		case http.MethodGet:
			resp, err := http.Get(creq.TargetURL)
			if err != nil {
				w.Write([]byte(fmt.Sprintf("HTTP GET to %q failed (%v)", creq.TargetURL, err)))
				return nil
			}
			d, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				w.Write([]byte(fmt.Sprintf("HTTP GET Read from %q failed (%v)", creq.TargetURL, err)))
				return nil
			}
			resp.Body.Close()
			_, err = w.Write(d)
			if err != nil {
				w.Write([]byte(fmt.Sprintf("HTTP GET Write to %q failed (%v)", creq.TargetURL, err)))
			}

		default:
			w.Write([]byte(fmt.Sprintf("method %q is not supported", creq.Method)))
		}

	default:
		http.Error(w, "Method Not Allowed", 405)
	}
	return nil
}
