package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	config_v2 "github.com/aws/aws-sdk-go-v2/config"
	sts_v2 "github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"go.uber.org/zap"
)

// NewV2 creates a new AWS session.
// Specify a custom endpoint for tests.
func NewV2(cfg *Config) (awsCfg aws_v2.Config, stsOutput *sts_v2.GetCallerIdentityOutput, err error) {
	if cfg == nil {
		return aws_v2.Config{}, nil, errors.New("got empty config")
	}
	if cfg.Logger == nil {
		return aws_v2.Config{}, nil, fmt.Errorf("missing logger")
	}
	if cfg.Partition == "" {
		return aws_v2.Config{}, nil, fmt.Errorf("missing partition")
	}
	if cfg.Region == "" {
		return aws_v2.Config{}, nil, fmt.Errorf("missing region")
	}

	optFns := []func(*config_v2.LoadOptions) error{
		(func(*config_v2.LoadOptions) error)(config_v2.WithRegion(cfg.Region)),
		(func(*config_v2.LoadOptions) error)(config_v2.WithLogger(toLoggerV2(cfg.Logger))),
	}
	if cfg.DebugAPICalls {
		lvl := aws_v2.LogSigning |
			aws_v2.LogRetries |
			aws_v2.LogRequest |
			aws_v2.LogRequestWithBody |
			aws_v2.LogResponse |
			aws_v2.LogResponseWithBody
		optFns = append(optFns, (func(*config_v2.LoadOptions) error)(config_v2.WithClientLogMode(lvl)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	awsCfg, err = config_v2.LoadDefaultConfig(ctx, optFns...)
	cancel()
	if err != nil {
		return aws_v2.Config{}, nil, fmt.Errorf("failed to load config %v", err)
	}

	stsCliV2 := sts_v2.NewFromConfig(awsCfg)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	stsOutput, err = stsCliV2.GetCallerIdentity(
		ctx,
		&sts_v2.GetCallerIdentityInput{},
	)
	cancel()
	if err != nil {
		return aws_v2.Config{}, nil, fmt.Errorf("failed to GetCallerIdentity %v", err)
	}

	cfg.Logger.Info("successfully get sts caller identity ",
		zap.String("partition", cfg.Partition),
		zap.String("region", cfg.Region),
		zap.String("account-id", stringVal(stsOutput.Account)),
		zap.String("user-id", stringVal(stsOutput.UserId)),
		zap.String("arn", stringVal(stsOutput.Arn)),
	)
	return awsCfg, stsOutput, nil
}

func stringVal(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// toLoggerV2 converts *zap.Logger to logging.Logger.
func toLoggerV2(lg *zap.Logger) logging.Logger {
	return &zapLoggerV2{lg}
}

type zapLoggerV2 struct {
	*zap.Logger
}

func (lg *zapLoggerV2) Logf(c logging.Classification, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	switch c {
	case logging.Warn:
		lg.Warn(msg)
	case logging.Debug:
		lg.Debug(msg)
	}
}
