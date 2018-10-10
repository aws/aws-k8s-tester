package s3

import (
	"github.com/aws/awstester/eksconfig"
)

type awsCli struct {
	cfg                 *eksconfig.Config
	bucketForTests      string
	bucketForAccessLogs string
}

// NewAWSCLI creates a new Plugin using AWS CLI.
func NewAWSCLI(cfg *eksconfig.Config) Plugin {
	return &awsCli{
		cfg:                 cfg,
		bucketForTests:      cfg.Tag,
		bucketForAccessLogs: cfg.Tag + "-access-logs",
	}
}

func (ac *awsCli) CreateBucketForAccessLogs() error {
	panic("not implemented")
}

func (ac *awsCli) BucketForTests() string {
	return ac.bucketForTests
}

func (ac *awsCli) BucketForAccessLogs() string {
	return ac.bucketForAccessLogs
}

func (ac *awsCli) UploadToBucketForTests(localPath, s3Path string) error {
	panic("not implemented")
}

func (ac *awsCli) DeleteBucket(bucket string) error {
	panic("not implemented")
}
