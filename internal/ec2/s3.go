package ec2

import (
	"bytes"
	"io/ioutil"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (md *embedded) toS3(localPath, s3Path string) error {
	bucket := md.cfg.Tag
	if _, ok := md.s3Buckets[bucket]; !ok {
		_, err := md.s3.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(bucket),
			CreateBucketConfiguration: &s3.CreateBucketConfiguration{
				LocationConstraint: aws.String(md.cfg.AWSRegion),
			},
			// TODO: enable this when open-sourced, to make all logs available to communities
			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// ACL: aws.String("public-read"),
			ACL: aws.String("private"),
		})
		if err != nil {
			exist := false
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case s3.ErrCodeBucketAlreadyExists:
					md.lg.Debug("bucket already exists", zap.String("bucket", bucket), zap.Error(err))
					exist = true
				case s3.ErrCodeBucketAlreadyOwnedByYou:
					md.lg.Debug("bucket already owned by me", zap.String("bucket", bucket), zap.Error(err))
					exist = true
				default:
					return err
				}
			}
			if !exist {
				return err
			}
		}
		md.s3Buckets[bucket] = struct{}{}
		md.lg.Info("created bucket", zap.String("bucket", bucket))

		h, _ := os.Hostname()
		tags := []*s3.Tag{{Key: aws.String("HOSTNAME"), Value: aws.String(h)}}
		if md.cfg.Tag != "" && md.cfg.ID != "" {
			tags = append(tags, &s3.Tag{Key: aws.String(md.cfg.Tag), Value: aws.String(md.cfg.ID)})
		}
		_, err = md.s3.PutBucketTagging(&s3.PutBucketTaggingInput{
			Bucket:  aws.String(bucket),
			Tagging: &s3.Tagging{TagSet: tags},
		})
		if err != nil {
			return err
		}
	}

	d, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}

	h, _ := os.Hostname()
	_, err = md.s3.PutObject(&s3.PutObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(s3Path),
		Body:    bytes.NewReader(d),
		Expires: aws.Time(time.Now().UTC().Add(24 * time.Hour)),

		// TODO: enable this when open-sourced, to make all logs available to communities
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// ACL: aws.String("public-read"),
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			md.cfg.Tag: aws.String(bucket),
			"HOSTNAME": aws.String(h),
		},
	})
	if err == nil {
		md.lg.Debug("uploaded",
			zap.String("bucket", bucket),
			zap.String("local-path", localPath),
			zap.String("remote-path", s3Path),
			zap.String("size", humanize.Bytes(uint64(len(d)))),
		)
	} else {
		md.lg.Warn("failed to upload",
			zap.String("bucket", bucket),
			zap.String("local-path", localPath),
			zap.String("remote-path", s3Path),
			zap.String("size", humanize.Bytes(uint64(len(d)))),
			zap.Error(err),
		)
	}
	return err
}

func (md *embedded) deleteBucket() error {
	bucket := md.cfg.Tag

	md.lg.Info("emptying bucket", zap.String("bucket", bucket))
	batcher := s3manager.NewBatchDeleteWithClient(md.s3)
	iter := &s3manager.DeleteListIterator{
		Bucket: aws.String(bucket),
		Paginator: request.Pagination{
			NewRequest: func() (*request.Request, error) {
				req, _ := md.s3.ListObjectsRequest(&s3.ListObjectsInput{
					Bucket: aws.String(bucket),
				})
				return req, nil
			},
		},
	}
	err := batcher.Delete(aws.BackgroundContext(), iter)
	if err != nil {
		md.lg.Warn("failed to empty bucket", zap.String("bucket", bucket), zap.Error(err))
		return err
	}
	md.lg.Info("emptied bucket", zap.String("bucket", bucket))

	md.lg.Info("deleting bucket", zap.String("bucket", bucket))
	_, err = md.s3.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		md.lg.Info("deleted bucket", zap.String("bucket", bucket))
		delete(md.s3Buckets, bucket)
	} else {
		md.lg.Warn("failed to delete bucket", zap.String("bucket", bucket), zap.Error(err))
	}
	return err
}
