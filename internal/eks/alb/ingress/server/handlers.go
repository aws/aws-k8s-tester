package server

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress/path"
	"github.com/aws/aws-k8s-tester/pkg/ctxhandler"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// responseBody is the default response body.
var responseBody = bytes.Repeat([]byte("0"), 10)

// NewMux returns a new HTTP request multiplexer with registered handlers.
func NewMux(ctx context.Context, lg *zap.Logger, routesN, responseN int) (*http.ServeMux, error) {
	responseBody = bytes.Repeat([]byte("0"), responseN)

	mux := http.NewServeMux()
	mux.Handle(path.Path, &ctxhandler.ContextAdapter{
		Logger:  lg,
		Ctx:     ctx,
		Handler: ctxhandler.ContextHandlerFunc(Handler),
	})
	mux.Handle(path.PathMetrics, promhttp.Handler())

	if routesN > 0 {
		handlers := make(map[string]ctxhandler.ContextHandlerFunc, routesN)
		for i := 0; i < routesN; i++ {
			handlers[path.Create(i)] = Handler
		}
		for p, h := range handlers {
			mux.Handle(p, &ctxhandler.ContextAdapter{
				Logger:  lg,
				Ctx:     ctx,
				Handler: h,
			})
			lg.Info("registered handler", zap.String("path", p))
		}
		lg.Info("finished handler registration", zap.Int("handlers", routesN))
	}
	return mux, nil
}

// Handler handles ingress traffic.
func Handler(ctx context.Context, w http.ResponseWriter, req *http.Request) (err error) {
	switch req.Method {
	case http.MethodGet:
		start := time.Now().UTC()

		// From, Method, Path
		promRecv.WithLabelValues(req.RemoteAddr, req.Method, req.RequestURI).Inc()

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(responseBody)

		// Path, To
		promSent.WithLabelValues(req.RequestURI, req.RemoteAddr).Observe(time.Now().UTC().Sub(start).Seconds())

	default:
		http.Error(w, "Method Not Allowed", 405)
	}
	return err
}
