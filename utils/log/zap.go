package log

import (
	"log"
	"sort"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	logger, err := GetDefaultZapLogger()
	if err != nil {
		log.Fatalf("Failed to initialize global logger, %v", err)
	}
	_ = zap.ReplaceGlobals(logger)
}

// GetDefaultZapLoggerConfig returns a new default zap logger configuration.
func GetDefaultZapLoggerConfig() zap.Config {
	return zap.Config{
		Level: zap.NewAtomicLevelAt(ConvertToZapLevel(DefaultLogLevel)),

		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},

		Encoding: "json",

		// copied from "zap.NewProductionEncoderConfig" with some updates
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},

		// Use "/dev/null" to discard all
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// GetDefaultZapLogger returns a new default logger.
func GetDefaultZapLogger() (*zap.Logger, error) {
	lcfg := GetDefaultZapLoggerConfig()
	return lcfg.Build()
}

// AddOutputPaths adds output paths to the existing output paths, resolving conflicts.
func AddOutputPaths(cfg zap.Config, outputPaths, errorOutputPaths []string) zap.Config {
	outputs := make(map[string]struct{})
	for _, v := range cfg.OutputPaths {
		outputs[v] = struct{}{}
	}
	for _, v := range outputPaths {
		outputs[v] = struct{}{}
	}
	outputSlice := make([]string, 0)
	if _, ok := outputs["/dev/null"]; ok {
		// "/dev/null" to discard all
		outputSlice = []string{"/dev/null"}
	} else {
		for k := range outputs {
			outputSlice = append(outputSlice, k)
		}
	}
	cfg.OutputPaths = outputSlice
	sort.Strings(cfg.OutputPaths)

	errOutputs := make(map[string]struct{})
	for _, v := range cfg.ErrorOutputPaths {
		errOutputs[v] = struct{}{}
	}
	for _, v := range errorOutputPaths {
		errOutputs[v] = struct{}{}
	}
	errOutputSlice := make([]string, 0)
	if _, ok := errOutputs["/dev/null"]; ok {
		// "/dev/null" to discard all
		errOutputSlice = []string{"/dev/null"}
	} else {
		for k := range errOutputs {
			errOutputSlice = append(errOutputSlice, k)
		}
	}
	cfg.ErrorOutputPaths = errOutputSlice
	sort.Strings(cfg.ErrorOutputPaths)

	return cfg
}
