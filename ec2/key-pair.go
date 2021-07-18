package ec2

import (
	"context"
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
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ec2_v2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	smithy "github.com/aws/smithy-go"
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
	output, err := ts.ec2APIV2.CreateKeyPair(
		context.Background(),
		&aws_ec2_v2.CreateKeyPairInput{
			KeyName: aws_v2.String(ts.cfg.RemoteAccessKeyName),
		})
	if err != nil {
		ev, ok := err.(awserr.Error)
		if ok && ev.Code() == "InvalidKeyPair.Duplicate" && fileutil.Exist(ts.cfg.RemoteAccessPrivateKeyPath) {
			ts.lg.Warn("key pair already created, private key locally exists, skipping EC2 key pair creation")
			return nil
		}
	}
	if aws_v2.ToString(output.KeyName) != ts.cfg.RemoteAccessKeyName {
		return fmt.Errorf("unexpected key name %q, expected %q", aws_v2.ToString(output.KeyName), ts.cfg.RemoteAccessKeyName)
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
		0400,
	); err != nil {
		return err
	}
	ts.lg.Info(
		"wrote EC2 private key on disk",
		zap.String("key-path", ts.cfg.RemoteAccessPrivateKeyPath),
	)

	if ts.cfg.S3.BucketName != "" {
		s3Key := path.Join(ts.cfg.Name, ts.cfg.RemoteAccessKeyName+".private.pem")
		_, err = ts.s3API.PutObject(&s3.PutObjectInput{
			Bucket: aws_v2.String(ts.cfg.S3.BucketName),
			Key:    aws_v2.String(s3Key),
			Body:   strings.NewReader(aws_v2.ToString(output.KeyMaterial)),

			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// vs. "public-read"
			ACL: aws_v2.String("private"),

			Metadata: map[string]*string{
				"Kind": aws_v2.String("aws-k8s-tester"),
				"User": aws_v2.String(user.Get()),
			},
		})
		if err == nil {
			ts.lg.Info("uploaded EC2 private key",
				zap.String("bucket", ts.cfg.S3.BucketName),
				zap.String("remote-path", s3Key),
			)
		} else {
			ts.lg.Warn("failed to upload EC2 private key",
				zap.String("bucket", ts.cfg.S3.BucketName),
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
	if _, ok := ts.cfg.DeletedResources[ts.cfg.RemoteAccessKeyName]; ok {
		return nil
	}

	err := os.RemoveAll(ts.cfg.RemoteAccessPrivateKeyPath)
	if err != nil {
		return err
	}
	ts.lg.Info("deleted EC2 private key on disk", zap.String("key-path", ts.cfg.RemoteAccessPrivateKeyPath))

	ts.lg.Info("deleting EC2 key pair", zap.String("key-pair-name", ts.cfg.RemoteAccessKeyName))
	_, err = ts.ec2APIV2.DeleteKeyPair(
		context.Background(),
		&aws_ec2_v2.DeleteKeyPairInput{
			KeyName: aws_v2.String(ts.cfg.RemoteAccessKeyName),
		},
	)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.lg.Warn("key pair already deleted")
				ts.cfg.DeletedResources[ts.cfg.RemoteAccessKeyName] = "RemoteAccessKeyName"
				ts.cfg.Sync()
				return nil
			}
		}
		return err
	}

	time.Sleep(time.Second)
	deleted := false
	for i := 0; i < 10; i++ {
		_, err = ts.ec2APIV2.DescribeKeyPairs(
			context.Background(),
			&aws_ec2_v2.DescribeKeyPairsInput{
				KeyNames: []string{ts.cfg.RemoteAccessKeyName},
			})
		if err == nil {
			time.Sleep(3 * time.Second)
			continue
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if strings.Contains(apiErr.ErrorCode(), "NotFound") {
				ts.lg.Warn("key pair already deleted")
				ts.cfg.DeletedResources[ts.cfg.RemoteAccessKeyName] = "RemoteAccessKeyName"
				ts.cfg.Sync()
				deleted = true
				break
			}
		}
		ts.lg.Warn("failed to describe key", zap.Error(err))
	}
	if !deleted {
		return fmt.Errorf("deleted EC2 key pair but %q still exists", ts.cfg.RemoteAccessKeyName)
	}
	ts.lg.Info("deleted EC2 key pair", zap.String("key-pair-name", ts.cfg.RemoteAccessKeyName))

	if ts.cfg.S3.BucketName != "" {
		s3Key := path.Join(ts.cfg.Name, ts.cfg.RemoteAccessKeyName+".private.pem")
		_, err = ts.s3API.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws_v2.String(ts.cfg.S3.BucketName),
			Key:    aws_v2.String(s3Key),
		})
		if err == nil {
			ts.lg.Info("deleted EC2 private key in S3",
				zap.String("bucket", ts.cfg.S3.BucketName),
				zap.String("remote-path", s3Key),
			)
		} else {
			ts.lg.Warn("failed to delete EC2 private key in S3",
				zap.String("bucket", ts.cfg.S3.BucketName),
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
