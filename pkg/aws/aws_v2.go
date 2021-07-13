package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	config_v2 "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go/logging"
	"go.uber.org/zap"
)

// NewV2 creates a new AWS session.
// Specify a custom endpoint for tests.
func NewV2(cfg *Config) (awsCfg aws_v2.Config, err error) {
	if cfg == nil {
		return aws_v2.Config{}, errors.New("got empty config")
	}
	if cfg.Logger == nil {
		return aws_v2.Config{}, fmt.Errorf("missing logger")
	}
	if cfg.Partition == "" {
		return aws_v2.Config{}, fmt.Errorf("missing partition")
	}
	if cfg.Region == "" {
		return aws_v2.Config{}, fmt.Errorf("missing region")
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

	if cfg.ResolverURL != "" {
		cfg.Logger.Info(
			"setting endpoint resolver for all services",
			zap.String("resolver-url", cfg.ResolverURL),
			zap.String("signing-name", cfg.SigningName),
		)
		opt := config_v2.WithEndpointResolver(aws_v2.EndpointResolverFunc(func(service string, region string) (aws_v2.Endpoint, error) {
			// v2 SDK does not support exported default resolver
			return aws_v2.Endpoint{
				URL:           cfg.ResolverURL,
				SigningName:   cfg.SigningName,
				SigningRegion: region,
				PartitionID:   "aws",
				SigningMethod: "",
				Source:        aws_v2.EndpointSourceCustom,
			}, nil
		}))
		optFns = append(optFns, opt)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	awsCfg, err = config_v2.LoadDefaultConfig(ctx, optFns...)
	cancel()
	if err != nil {
		return aws_v2.Config{}, fmt.Errorf("failed to load config %v", err)
	}

	return awsCfg, nil
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
