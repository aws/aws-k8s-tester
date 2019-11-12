package eks

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

func (ts *Tester) createKeyPair() (err error) {
	if ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName == "" {
		return errors.New("cannot create key pair without key name")
	}
	if ts.cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath == "" {
		return errors.New("cannot create key pair without private key path")
	}

	ts.lg.Info("creating a new key pair", zap.String("key-pair-name", ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName))

	now := time.Now().UTC()

	var output *ec2.CreateKeyPairOutput
	output, err = ts.ec2API.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: aws.String(ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName),
	})
	if err != nil {
		return err
	}

	if *output.KeyName != ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName {
		return fmt.Errorf("unexpected key name %q, expected %q", *output.KeyName, ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName)
	}
	if err = os.MkdirAll(filepath.Dir(ts.cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath), 0700); err != nil {
		return err
	}
	if err = ioutil.WriteFile(
		ts.cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath,
		[]byte(*output.KeyMaterial),
		0400,
	); err != nil {
		return err
	}

	ts.lg.Info(
		"created a new key pair",
		zap.String("key-name", ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteKeyPair() error {
	if ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName == "" {
		return errors.New("cannot delete key pair without key name")
	}
	defer os.RemoveAll(ts.cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath)

	ts.lg.Info("deleting a key pair", zap.String("key-pair-name", ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName))

	_, err := ts.ec2API.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName),
	})
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	deleted := false
	for i := 0; i < 10; i++ {
		_, err = ts.ec2API.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
			KeyNames: aws.StringSlice([]string{ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName}),
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
	if deleted {
		ts.lg.Info("deleted a key pair", zap.String("key-pair-name", ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName))
		return nil
	}
	return fmt.Errorf("deleted key pair but %q still exists", ts.cfg.Parameters.ManagedNodeGroupSSHKeyPairName)
}
