package ec2

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (ts *Tester) createS3() (err error) {
	if ts.cfg.S3BucketCreate {
		if ts.cfg.S3BucketName == "" {
			return errors.New("empty S3 bucket name")
		}
		var retry bool
		for i := 0; i < 5; i++ {
			retry, err = ts.createBucket()
			if err == nil {
				break
			}
			if retry {
				ts.lg.Warn("failed to create bucket; retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			return err
		}
		if err != nil {
			return err
		}
	} else {
		ts.lg.Info("skipping S3 bucket creation")
	}
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 bucket creation")
		return nil
	}

	testKey := path.Join(ts.cfg.Name, "test-"+getTS()+".log")
	_, err = ts.s3API.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Key:    aws.String(testKey),
		Body:   strings.NewReader("Hello World!"),

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
		},
	})
	if err == nil {
		ts.lg.Info("uploaded",
			zap.String("bucket", ts.cfg.S3BucketName),
			zap.String("remote-path", testKey),
		)
	} else {
		ts.lg.Warn("failed to upload",
			zap.String("bucket", ts.cfg.S3BucketName),
			zap.String("remote-path", testKey),
			zap.Error(err),
		)
	}
	return ts.cfg.Sync()
}

func (ts *Tester) createBucket() (bool, error) {
	if !ts.cfg.S3BucketCreate {
		ts.lg.Info("skipping S3 bucket creation", zap.String("name", ts.cfg.S3BucketName))
		return false, nil
	}
	ts.lg.Info("creating S3 bucket", zap.String("name", ts.cfg.S3BucketName))
	_, err := ts.s3API.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(ts.cfg.Region),
		},
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),
	})
	alreadyExist := false
	if err != nil {
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				ts.lg.Warn("bucket already exists", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				alreadyExist, err = true, nil
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				ts.lg.Warn("bucket already owned by me", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				alreadyExist, err = true, nil
			default:
				if strings.Contains(err.Error(), "OperationAborted: A conflicting conditional operation is currently in progress against this resource. Please try again.") ||
					request.IsErrorRetryable(err) ||
					request.IsErrorThrottle(err) {
					return true, err
				}
				ts.lg.Warn("failed to create bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.String("code", aerr.Code()), zap.Error(err))
				return false, err
			}
		}
		if !alreadyExist {
			ts.lg.Warn("failed to create bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.String("type", reflect.TypeOf(err).String()), zap.Error(err))
			return false, err
		}
	}
	if alreadyExist {
		return false, nil
	}
	ts.lg.Info("created S3 bucket", zap.String("bucket", ts.cfg.S3BucketName))

	_, err = ts.s3API.PutBucketTagging(&s3.PutBucketTaggingInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Tagging: &s3.Tagging{TagSet: []*s3.Tag{
			{Key: aws.String("Kind"), Value: aws.String("aws-k8s-tester")},
			{Key: aws.String("Creation"), Value: aws.String(time.Now().String())},
		}},
	})
	if err != nil {
		return true, err
	}

	if ts.cfg.S3BucketLifecycleExpirationDays > 0 {
		_, err = ts.s3API.PutBucketLifecycle(&s3.PutBucketLifecycleInput{
			Bucket: aws.String(ts.cfg.S3BucketName),
			LifecycleConfiguration: &s3.LifecycleConfiguration{
				Rules: []*s3.Rule{
					{
						Prefix: aws.String(ts.cfg.Name),
						AbortIncompleteMultipartUpload: &s3.AbortIncompleteMultipartUpload{
							DaysAfterInitiation: aws.Int64(ts.cfg.S3BucketLifecycleExpirationDays),
						},
						Expiration: &s3.LifecycleExpiration{
							Days: aws.Int64(ts.cfg.S3BucketLifecycleExpirationDays),
						},
						ID:     aws.String(fmt.Sprintf("ObjectLifecycleOf%vDays", ts.cfg.S3BucketLifecycleExpirationDays)),
						Status: aws.String("Enabled"),
					},
				},
			},
		})
		if err != nil {
			return true, err
		}
	}

	return false, ts.cfg.Sync()
}

func (ts *Tester) deleteS3() error {
	if !ts.cfg.S3BucketCreate {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName))
		return nil
	}
	if ts.cfg.S3BucketCreateKeep {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName), zap.Bool("s3-bucket-create-keep", ts.cfg.S3BucketCreateKeep))
		return nil
	}

	ts.lg.Info("emptying bucket", zap.String("bucket", ts.cfg.S3BucketName))
	batcher := s3manager.NewBatchDeleteWithClient(ts.s3API)
	iter := &s3manager.DeleteListIterator{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Paginator: request.Pagination{
			NewRequest: func() (*request.Request, error) {
				req, _ := ts.s3API.ListObjectsRequest(&s3.ListObjectsInput{
					Bucket: aws.String(ts.cfg.S3BucketName),
				})
				return req, nil
			},
		},
	}
	err := batcher.Delete(aws.BackgroundContext(), iter)
	if err != nil { // https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				ts.lg.Info("no such bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				return nil
			}
		}
		ts.lg.Warn("failed to empty bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
		return err
	}
	ts.lg.Info("emptied bucket", zap.String("bucket", ts.cfg.S3BucketName))

	ts.lg.Info("deleting bucket", zap.String("bucket", ts.cfg.S3BucketName))
	_, err = ts.s3API.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				ts.lg.Info("no such bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				return nil
			}
		}
		ts.lg.Warn("failed to delete bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
	}

	ts.lg.Info("deleted bucket", zap.String("bucket", ts.cfg.S3BucketName))
	return nil
}

func getTS() string {
	now := time.Now()
	return fmt.Sprintf(
		"%04d%02d%02d%02d%02d",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		now.Second(),
	)
}

func (ts *Tester) uploadToS3() (err error) {
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 uploads; s3 bucket name is empty")
		return nil
	}

	if err = uploadFileToS3(
		ts.lg,
		ts.s3API,
		ts.cfg.S3BucketName,
		path.Join(ts.cfg.Name, "aws-k8s-tester-ec2.config.yaml"),
		ts.cfg.ConfigPath,
	); err != nil {
		return err
	}

	logFilePath := ""
	for _, fpath := range ts.cfg.LogOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if fileutil.Exist(logFilePath) {
		if err = uploadFileToS3(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "aws-k8s-tester-ec2.log"),
			logFilePath,
		); err != nil {
			return err
		}
	}

	if fileutil.Exist(ts.cfg.RoleCFNStackYAMLFilePath) {
		if err = uploadFileToS3(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "cfn", "aws-k8s-tester-ec2.role.cfn.yaml"),
			ts.cfg.RoleCFNStackYAMLFilePath,
		); err != nil {
			return err
		}
	}

	if fileutil.Exist(ts.cfg.VPCCFNStackYAMLFilePath) {
		if err = uploadFileToS3(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "cfn", "aws-k8s-tester-ec2.vpc.cfn.yaml"),
			ts.cfg.VPCCFNStackYAMLFilePath,
		); err != nil {
			return err
		}
	}

	for _, cur := range ts.cfg.ASGs {
		if fileutil.Exist(cur.SSMDocumentCFNStackYAMLFilePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", filepath.Base(cur.SSMDocumentCFNStackYAMLFilePath)),
				cur.SSMDocumentCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(cur.ASGCFNStackYAMLFilePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", filepath.Base(cur.ASGCFNStackYAMLFilePath)),
				cur.ASGCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func uploadFileToS3(lg *zap.Logger, s3API s3iface.S3API, bucketName string, s3Key string, fpath string) error {
	d, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	_, err = s3API.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
		Body:   bytes.NewReader(d),

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
		},
	})
	if err == nil {
		lg.Info("uploaded",
			zap.String("bucket", bucketName),
			zap.String("remote-path", s3Key),
			zap.String("size", humanize.Bytes(uint64(len(d)))),
		)
	} else {
		lg.Warn("failed to upload",
			zap.String("bucket", bucketName),
			zap.String("remote-path", s3Key),
			zap.Error(err),
		)
	}
	return err
}
