package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// NewWithStderrWriter creates a new logger and multi-writer with os.Stderr.
// The returned file object is the log file.
// The log file must be specified with extension ".log".
// If the logOutputs is "stderr", it just returns the os.Stderr.
func NewWithStderrWriter(logLevel string, logOutputs []string) (lg *zap.Logger, wr io.Writer, logFile *os.File, err error) {
	lcfg := GetDefaultZapLoggerConfig()
	if len(logOutputs) == 0 {
		wr = os.Stderr
	}
	if len(logOutputs) == 1 {
		o := strings.ToLower(logOutputs[0])
		if o == "stderr" {
			wr = os.Stderr
		}
		if o == "stdout" {
			wr = os.Stdout
		}
	}
	if len(logOutputs) > 1 {
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
			fmt.Fprintf(os.Stderr, "[WARN] failed to open log file %q (%v) -- ignoring log file\n", logFilePath, err)
			wr = io.MultiWriter(os.Stderr)
			lcfg = AddOutputPaths(lcfg, nil, nil)
		} else {
			wr = io.MultiWriter(os.Stderr, logFile)
			lcfg = AddOutputPaths(lcfg, logOutputs, logOutputs)
		}
	}

	lcfg.Level = zap.NewAtomicLevelAt(ConvertToZapLevel(logLevel))

	lg, err = lcfg.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	return lg, wr, logFile, nil
}
