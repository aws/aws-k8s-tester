// Package ssm implements common SSM utilities.
package ssm

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"go.uber.org/zap"
)

// FetchConfig is the SSM fetch configuration.
type FetchConfig struct {
	Logger        *zap.Logger
	SSMAPI        ssmiface.SSMAPI
	CommandID     string
	DocumentName  string
	InvokedAfter  time.Time
	BatchLimit    int64
	BatchInterval time.Duration
}

const rfc3339Micro = "2006-01-02T15:04:05.999Z07:00"

// FetchOutputs downloads SSM doc run outputs.
// It only returns the invocation whose status is "Success".
// e.g. aws ssm list-command-invocations --details --filter key=DocumentName,value=ResourceCounterSSMDocDevStack
func FetchOutputs(cfg FetchConfig) (ivs []*ssm.CommandInvocation, err error) {
	if cfg.BatchLimit > 50 {
		return nil, fmt.Errorf("batch limit exceeded 50, got %v", cfg.BatchLimit)
	}
	cfg.Logger.Info("fetching SSM doc outputs", zap.String("document-name", cfg.DocumentName), zap.Int64("batch-limit", cfg.BatchLimit))
	input := &ssm.ListCommandInvocationsInput{
		Details:    aws.Bool(true),
		MaxResults: aws.Int64(cfg.BatchLimit),
		Filters: []*ssm.CommandFilter{
			{
				Key:   aws.String("DocumentName"),
				Value: aws.String(cfg.DocumentName),
			},
			{
				Key:   aws.String("InvokedAfter"),
				Value: aws.String(cfg.InvokedAfter.Format(rfc3339Micro)),
			},
		},
	}
	if cfg.CommandID != "" {
		input.CommandId = aws.String(cfg.CommandID)
	}
	var output *ssm.ListCommandInvocationsOutput
	for {
		output, err = cfg.SSMAPI.ListCommandInvocations(input)
		if err != nil {
			cfg.Logger.Warn("failed to fetch SSM doc outputs", zap.Error(err))
			return nil, err
		}
		rs := output.CommandInvocations
		n := len(rs)
		if n == 0 {
			break
		}
		for _, rv := range rs {
			if aws.StringValue(rv.Status) == "Success" {
				ivs = append(ivs, rv)
			}
		}
		token := aws.StringValue(output.NextToken)
		input.NextToken = aws.String(token)
		cfg.Logger.Info("received SSM command invocation outputs", zap.Int("received", n), zap.Int("total", len(ivs)))
		if token == "" {
			break
		}
		time.Sleep(cfg.BatchInterval)
	}
	cfg.Logger.Info("fetching SSM doc outputs", zap.Int("total", len(ivs)))
	return ivs, nil
}
