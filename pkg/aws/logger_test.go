package aws

import (
	"testing"

	"go.uber.org/zap"
)

func Test_toLogger(t *testing.T) {
	lg := toLogger(zap.NewExample())
	lg.Log("hello", "world", 1)
}
