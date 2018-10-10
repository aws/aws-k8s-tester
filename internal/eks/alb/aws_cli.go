package alb

import "github.com/aws/awstester/eksconfig"

type awsCli struct {
	cfg *eksconfig.Config
}

// NewAWSCLI creates a new Plugin using AWS CLI.
func NewAWSCLI(cfg *eksconfig.Config) Plugin {
	return &awsCli{cfg: cfg}
}
