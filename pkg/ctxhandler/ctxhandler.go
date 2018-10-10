// Package ctxhandler implements context handler.
package ctxhandler

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

// ContextHandler handles ServeHTTP with context.
type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request) error
}

// ContextHandlerFunc defines HandlerFunc function signature to wrap context.
type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

// ServeHTTPContext serve HTTP requests with context.
func (f ContextHandlerFunc) ServeHTTPContext(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	return f(ctx, w, req)
}

// ContextAdapter wraps context handler.
type ContextAdapter struct {
	Logger  *zap.Logger
	Ctx     context.Context
	Handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if err := ca.Handler.ServeHTTPContext(ca.Ctx, w, req); err != nil {
		ca.Logger.Warn("failed to serve", zap.String("method", req.Method), zap.String("path", req.URL.Path), zap.Error(err))
	}
}
