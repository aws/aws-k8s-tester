package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"go.uber.org/zap"
)

// NewV2 creates a new AWS session.
// Specify a custom endpoint for tests.
func NewV2(cfg *Config) (awsCfg aws.Config, stsOutput *sts.GetCallerIdentityOutput, err error) {
	if cfg == nil {
		return aws.Config{}, nil, errors.New("got empty config")
	}
	if cfg.Logger == nil {
		return aws.Config{}, nil, fmt.Errorf("missing logger")
	}
	if cfg.Partition == "" {
		return aws.Config{}, nil, fmt.Errorf("missing partition")
	}
	if cfg.Region == "" {
		return aws.Config{}, nil, fmt.Errorf("missing region")
	}

	optFns := []func(*config.LoadOptions) error{
		(func(*config.LoadOptions) error)(config.WithRegion(cfg.Region)),
		(func(*config.LoadOptions) error)(config.WithLogger(toLoggerV2(cfg.Logger))),
	}
	if cfg.DebugAPICalls {
		lvl := aws.LogSigning |
			aws.LogRetries |
			aws.LogRequest |
			aws.LogRequestWithBody |
			aws.LogResponse |
			aws.LogResponseWithBody
		optFns = append(optFns, (func(*config.LoadOptions) error)(config.WithClientLogMode(lvl)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	awsCfg, err = config.LoadDefaultConfig(ctx, optFns...)
	cancel()
	if err != nil {
		return aws.Config{}, nil, fmt.Errorf("failed to load config %v", err)
	}

	stsCli := sts.NewFromConfig(awsCfg)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	stsOutput, err = stsCli.GetCallerIdentity(
		ctx,
		&sts.GetCallerIdentityInput{},
	)
	cancel()
	if err != nil {
		return aws.Config{}, nil, fmt.Errorf("failed to GetCallerIdentity %v", err)
	}

	cfg.Logger.Info("successfully get sts caller identity",
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
