package awssdk

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"k8s.io/klog/v2"
)

// NewConfig returns an AWS SDK config
// It will panic if the cnfig cannot be created
func NewConfig() aws.Config {
	c, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		klog.Fatalf("failed to create AWS SDK config: %v", err)
	}
	return c
}
