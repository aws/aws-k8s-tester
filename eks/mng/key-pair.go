package mng

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// SECURITY NOTE: MAKE SURE PRIVATE KEY NEVER GETS UPLOADED TO CLOUD STORAGE AND DLETE AFTER USE!!!

func (ts *tester) createKeyPair() (err error) {
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName == "" {
		return errors.New("cannot create key pair without key name")
	}
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath == "" {
		return errors.New("cannot create key pair without private key path")
	}

	ts.cfg.Logger.Info("creating a new key pair", zap.String("key-pair-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName))

	now := time.Now()

	var output *ec2.CreateKeyPairOutput
	output, err = ts.cfg.EC2API.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName),
	})
	if err != nil {
		return err
	}

	if *output.KeyName != ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName {
		return fmt.Errorf("unexpected key name %q, expected %q", *output.KeyName, ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName)
	}
	if err = os.MkdirAll(filepath.Dir(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath), 0700); err != nil {
		return err
	}
	if err = ioutil.WriteFile(
		ts.cfg.EKSConfig.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath,
		[]byte(*output.KeyMaterial),
		0400,
	); err != nil {
		return err
	}

	ts.cfg.Logger.Info(
		"created a new key pair",
		zap.String("key-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName),
		zap.String("request-started", humanize.RelTime(now, time.Now(), "ago", "from now")),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteKeyPair() error {
	if ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName == "" {
		return errors.New("cannot delete key pair without key name")
	}
	defer os.RemoveAll(ts.cfg.EKSConfig.AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath)

	ts.cfg.Logger.Info("deleting a key pair", zap.String("key-pair-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName))

	_, err := ts.cfg.EC2API.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName),
	})
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	deleted := false
	for i := 0; i < 10; i++ {
		_, err = ts.cfg.EC2API.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
			KeyNames: aws.StringSlice([]string{ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName}),
		})
		if err == nil {
			time.Sleep(3 * time.Second)
			continue
		}
		if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
			ts.cfg.Logger.Warn("failed to describe key pair, retrying...", zap.Error(err))
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
		return fmt.Errorf("deleted key pair but %q still exists", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName)
	}

	ts.cfg.Logger.Info("deleted a key pair", zap.String("key-pair-name", ts.cfg.EKSConfig.AddOnManagedNodeGroups.SSHKeyPairName))
	return nil
}
