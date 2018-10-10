// Package s3 implements S3 plugin.
package s3

// Plugin defines S3 plugin.
type Plugin interface {
	CreateBucketForAccessLogs() error
	BucketForAccessLogs() string
	BucketForTests() string
	UploadToBucketForTests(localPath, s3Path string) error
	DeleteBucket(bucket string) error
}
