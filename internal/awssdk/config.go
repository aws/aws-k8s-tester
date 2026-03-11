package awssdk

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// NewConfig returns an AWS SDK config
// It will panic if the cnfig cannot be created
func NewConfig() aws.Config {
	c, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		slog.Error("failed to create AWS SDK config", "error", err)
		panic(err)
	}
	return c
}
