package httputil

import (
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// CheckGet retries until HTTP response returns the expected output.
func CheckGet(lg *zap.Logger, u, exp string, retries int, interval time.Duration, stopc chan struct{}) bool {
	for retries > 0 {
		select {
		case <-stopc:
			return false
		default:
		}
		resp, err := http.Get(u)
		if err != nil {
			lg.Warn(
				"HTTP Get failed",
				zap.String("endpoint", u),
				zap.Error(err),
			)
			retries--
			time.Sleep(interval)
			continue
		}

		d, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			lg.Warn(
				"failed to read from HTTP Response",
				zap.String("endpoint", u),
				zap.Error(err),
			)
			retries--
			time.Sleep(interval)
			continue
		}
		resp.Body.Close()

		if exp != "" && string(d) != exp {
			lg.Warn(
				"unexpected data from HTTP Response",
				zap.String("endpoint", u),
				zap.Int("expected-bytes", len(exp)),
				zap.Int("response-bytes", len(d)),
			)
			retries--
			time.Sleep(interval)
			continue
		}

		lg.Info("HTTP Get success", zap.String("endpoint", u), zap.Int("response-size", len(d)))
		return true
	}
	return false
}
