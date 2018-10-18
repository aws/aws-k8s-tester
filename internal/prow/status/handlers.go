package status

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aws/awstester/pkg/ctxhandler"

	"go.uber.org/zap"
)

const (
	// Path to server upstream test status frontend.
	Path = "/prow-status"
	// PathReadiness to serve readiness.
	PathReadiness = "/prow-status-readiness"
	// PathLiveness to serve liveness.
	PathLiveness = "/prow-status-liveness"
)

type key int

const statusKey key = 0

// NewMux returns HTTP request multiplexer with registered handlers.
func NewMux(ctx context.Context, lg *zap.Logger) *http.ServeMux {
	s := newStatus(lg)

	lg.Info("refreshing on initial start")
	s.refresh()
	lg.Info("refreshed on initial start")

	ctx = context.WithValue(ctx, statusKey, s)
	mux := http.NewServeMux()
	mux.Handle(Path, &ctxhandler.ContextAdapter{
		Logger:  lg,
		Ctx:     ctx,
		Handler: ctxhandler.ContextHandlerFunc(handlerPath),
	})
	mux.Handle(PathReadiness, &ctxhandler.ContextAdapter{
		Logger:  lg,
		Ctx:     ctx,
		Handler: ctxhandler.ContextHandlerFunc(handlerPathReadiness),
	})
	mux.Handle(PathLiveness, &ctxhandler.ContextAdapter{
		Logger:  lg,
		Ctx:     ctx,
		Handler: ctxhandler.ContextHandlerFunc(handlerPathLiveness),
	})
	return mux
}

func handlerPath(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	switch req.Method {
	case http.MethodGet:
		s := ctx.Value(statusKey).(*status)
		s.refresh()
		s.statusMu.RLock()
		txt := s.statusHTMLHead +
			s.statusHTMLUpdateMsg +
			s.statusHTMLGitRows +
			s.statusHTMLJobRows +
			s.statusHTMLEnd
		s.statusMu.RUnlock()
		w.Write([]byte(txt))
		return nil

	default:
		http.Error(w, "Method Not Allowed", 405)
		return fmt.Errorf("Method %q Not Allowed", req.Method)
	}
}

func handlerPathReadiness(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	switch req.Method {
	case http.MethodGet:
		s := ctx.Value(statusKey).(*status)
		s.mu.RLock()
		ok := len(s.all) > 0
		s.mu.RUnlock()
		if ok {
			w.WriteHeader(http.StatusOK)
		}
		return nil

	default:
		http.Error(w, "Method Not Allowed", 405)
		return fmt.Errorf("Method %q Not Allowed", req.Method)
	}
}

func handlerPathLiveness(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	switch req.Method {
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("LIVE\n"))
		return err

	default:
		http.Error(w, "Method Not Allowed", 405)
		return fmt.Errorf("Method %q Not Allowed", req.Method)
	}
}
