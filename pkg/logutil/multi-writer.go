package logutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// NewWithStderrWriter creates a new logger and multi-writer with os.Stderr.
// The returned file object is the log file.
// The log file must be specified with extension ".log".
func NewWithStderrWriter(logLevel string, logOutputs []string) (lg *zap.Logger, wr io.Writer, logFile *os.File, err error) {
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
	var lcfg zap.Config
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] failed to open log file %q (%v) -- ignoring log file\n", logFilePath, err)
		wr = io.MultiWriter(os.Stderr)
		lcfg = AddOutputPaths(GetDefaultZapLoggerConfig(), nil, nil)
		// return nil, nil, nil, err
	} else {
		wr = io.MultiWriter(os.Stderr, logFile)
		lcfg = AddOutputPaths(GetDefaultZapLoggerConfig(), logOutputs, logOutputs)
	}

	lcfg.Level = zap.NewAtomicLevelAt(ConvertToZapLevel(logLevel))
	lg, err = lcfg.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	return lg, wr, logFile, nil
}
