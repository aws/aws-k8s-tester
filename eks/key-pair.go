package eks

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// SECURITY NOTE:
// MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO "PUBLIC" CLOUD STORAGE
// MAKE SURE TO DELETE AFTER USE!!!

func (ts *Tester) createKeyPair() (err error) {
	if !ts.cfg.RemoteAccessKeyCreate {
		ts.lg.Info("skipping creating EC2 key pair")
		return nil
	}
	if ts.cfg.RemoteAccessKeyName == "" {
		return errors.New("cannot create EC2 key pair without key name")
	}
	if ts.cfg.RemoteAccessPrivateKeyPath == "" {
		return errors.New("cannot create EC2 key pair without private key path")
	}

	now := time.Now()

	ts.lg.Info("creating EC2 key pair", zap.String("key-pair-name", ts.cfg.RemoteAccessKeyName))
	var output *ec2.CreateKeyPairOutput
	output, err = ts.ec2API.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: aws.String(ts.cfg.RemoteAccessKeyName),
	})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if ok && ev.Code() == "InvalidKeyPair.Duplicate" && fileutil.Exist(ts.cfg.RemoteAccessPrivateKeyPath) {
			ts.lg.Warn("key pair already created, private key locally exists, skipping EC2 key pair creation")
			return nil
		}
		return err
	}
	if aws.StringValue(output.KeyName) != ts.cfg.RemoteAccessKeyName {
		return fmt.Errorf("unexpected key name %q, expected %q", aws.StringValue(output.KeyName), ts.cfg.RemoteAccessKeyName)
	}
	ts.lg.Info(
		"created EC2 key pair",
		zap.String("key-name", ts.cfg.RemoteAccessKeyName),
		zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
	)

	if err = os.MkdirAll(filepath.Dir(ts.cfg.RemoteAccessPrivateKeyPath), 0700); err != nil {
		return err
	}
	if err = ioutil.WriteFile(
		ts.cfg.RemoteAccessPrivateKeyPath,
		[]byte(*output.KeyMaterial),
		0600, // grant write permission in case of overwrites
	); err != nil {
		return err
	}
	ts.lg.Info("wrote EC2 private key on disk", zap.String("key-path", ts.cfg.RemoteAccessPrivateKeyPath))

	if ts.cfg.S3BucketName != "" {
		s3Key := path.Join(ts.cfg.Name, ts.cfg.RemoteAccessKeyName+".private.pem")
		_, err = ts.s3API.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(ts.cfg.S3BucketName),
			Key:    aws.String(s3Key),
			Body:   strings.NewReader(aws.StringValue(output.KeyMaterial)),

			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// vs. "public-read"
			ACL: aws.String("private"),

			Metadata: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
				"User": aws.String(user.Get()),
			},
		})
		if err == nil {
			ts.lg.Info("uploaded EC2 private key",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
			)
		} else {
			ts.lg.Warn("failed to upload EC2 private key",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.Error(err),
			)
		}
	} else {
		ts.lg.Info("skipping EC2 private key uploads")
	}
	return err
}

func (ts *Tester) deleteKeyPair() error {
	if !ts.cfg.RemoteAccessKeyCreate {
		ts.lg.Info("skipping deleting EC2 key pair")
		return nil
	}
	if ts.cfg.RemoteAccessKeyName == "" {
		return errors.New("cannot delete EC2 key pair without key name")
	}

	err := os.RemoveAll(ts.cfg.RemoteAccessPrivateKeyPath)
	if err != nil {
		return err
	}
	ts.lg.Info("deleted EC2 private key on disk", zap.String("key-path", ts.cfg.RemoteAccessPrivateKeyPath))

	ts.lg.Info("deleting EC2 key pair", zap.String("key-pair-name", ts.cfg.RemoteAccessKeyName))
	_, err = ts.ec2API.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(ts.cfg.RemoteAccessKeyName),
	})
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	deleted := false
	for i := 0; i < 10; i++ {
		_, err = ts.ec2API.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
			KeyNames: aws.StringSlice([]string{ts.cfg.RemoteAccessKeyName}),
		})
		if err == nil {
			time.Sleep(3 * time.Second)
			continue
		}
		if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
			ts.lg.Warn("failed to describe key pair, retrying...", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "InvalidKeyPair.NotFound" {
			deleted = true
			break
		}
	}
	if !deleted {
		return fmt.Errorf("deleted key pair but %q still exists", ts.cfg.RemoteAccessKeyName)
	}
	ts.lg.Info("deleted EC2 key pair", zap.String("key-pair-name", ts.cfg.RemoteAccessKeyName))

	if ts.cfg.S3BucketName != "" {
		s3Key := path.Join(ts.cfg.Name, ts.cfg.RemoteAccessKeyName+".private.pem")
		_, err = ts.s3API.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(ts.cfg.S3BucketName),
			Key:    aws.String(s3Key),
		})
		if err == nil {
			ts.lg.Info("deleted EC2 private key in S3",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
			)
		} else {
			ts.lg.Warn("failed to delete EC2 private key in S3",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.Error(err),
			)
			return err
		}
	} else {
		ts.lg.Info("skipping S3 EC2 private key clean-up")
	}

	return nil
}
