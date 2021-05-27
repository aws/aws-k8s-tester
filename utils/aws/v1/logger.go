package v1

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
)

// toLogger converts *zap.Logger to aws.Logger.
func toLogger(lg *zap.Logger) aws.Logger {
	return &zapLogger{lg}
}

type zapLogger struct {
	*zap.Logger
}

func (lg *zapLogger) Log(ss ...interface{}) {
	ms := make([]string, len(ss))
	for i := range ss {
		ms[i] = fmt.Sprintf("%v", ss[i])
	}
	lg.Info(strings.Join(ms, " "))
}
