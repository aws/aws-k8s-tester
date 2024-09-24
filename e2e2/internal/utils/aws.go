package utils

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// NewConfig returns an AWS SDK config
// It will panic if the cnfig cannot be created
func NewConfig() (aws.Config, error) {
	c, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create AWS SDK config: %v", err)
	}
	return c, nil
}
