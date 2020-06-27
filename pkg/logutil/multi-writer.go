package logutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// NewWithStderrWriter creates a new logger with stderr multi writer.
// The returned file object is the log file.
// The log file must be specified with extension ".log".
func NewWithStderrWriter(logLevel string, logOutputs []string) (lg *zap.Logger, wr io.Writer, logFile *os.File, err error) {
	lcfg := AddOutputPaths(GetDefaultZapLoggerConfig(), logOutputs, logOutputs)
	lcfg.Level = zap.NewAtomicLevelAt(ConvertToZapLevel(logLevel))
	lg, err = lcfg.Build()
	if err != nil {
		return nil, nil, nil, err
	}

	logFilePath := ""
	for _, fpath := range logOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if logFilePath == "" {
		return nil, nil, nil, fmt.Errorf(".log file not found %v", logOutputs)
	}
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		return nil, nil, nil, err
	}

	wr = io.MultiWriter(os.Stderr, logFile)
	return lg, wr, logFile, nil
}
