// Package zaputil implements various zap.Logger utilities.
package zaputil

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new zap.Logger.
func New(debug bool, outputs []string) (*zap.Logger, error) {
	logLvl := zap.InfoLevel
	if debug {
		logLvl = zap.DebugLevel
	}
	lcfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(logLvl),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},

		// 'json' or 'console'
		Encoding: "console",

		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     iso8601UTCTimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      outputs,
		ErrorOutputPaths: outputs,
	}
	return lcfg.Build()
}

func iso8601UTCTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z0700"))
}
