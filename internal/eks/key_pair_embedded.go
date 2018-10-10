package eks

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// SECURITY NOTE: we do not need private key for tests,
// so we do not even save the private key when created

func (md *embedded) createKeyPair() (err error) {
	if md.cfg.ClusterState.CFStackNodeGroupKeyPairName == "" {
		return errors.New("cannot create key pair without key name")
	}
	if md.cfg.ClusterState.CFStackNodeGroupKeyPairPrivateKeyPath == "" {
		return errors.New("cannot create key pair without private key path")
	}

	now := time.Now().UTC()

	var output *ec2.CreateKeyPairOutput
	output, err = md.ec2.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: aws.String(md.cfg.ClusterState.CFStackNodeGroupKeyPairName),
	})
	if err != nil {
		return err
	}
	md.cfg.ClusterState.StatusKeyPairCreated = true
	md.cfg.Sync()

	if *output.KeyName != md.cfg.ClusterState.CFStackNodeGroupKeyPairName {
		return fmt.Errorf("unexpected key name %q, expected %q", *output.KeyName, md.cfg.ClusterState.CFStackNodeGroupKeyPairName)
	}

	md.lg.Info(
		"created key pair",
		zap.String("key-name", md.cfg.ClusterState.CFStackNodeGroupKeyPairName),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	return md.cfg.Sync()
}

func (md *embedded) deleteKeyPair() error {
	if !md.cfg.ClusterState.StatusKeyPairCreated {
		return nil
	}
	defer func() {
		md.cfg.ClusterState.StatusKeyPairCreated = false
		md.cfg.Sync()
	}()

	if md.cfg.ClusterState.CFStackNodeGroupKeyPairName == "" {
		return errors.New("cannot delete key pair without key name")
	}

	now := time.Now().UTC()

	_, err := md.ec2.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(md.cfg.ClusterState.CFStackNodeGroupKeyPairName),
	})
	if err != nil {
		return err
	}

	time.Sleep(time.Second)

	_, err = md.ec2.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: aws.StringSlice([]string{md.cfg.ClusterState.CFStackNodeGroupKeyPairName}),
	})
	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if ok && awsErr.Code() == "InvalidKeyPair.NotFound" {
			md.lg.Info(
				"deleted key pair",
				zap.String("key-name", md.cfg.ClusterState.CFStackNodeGroupKeyPairName),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			return nil
		}
		return err
	}
	return fmt.Errorf("deleted key pair but %q still exists", md.cfg.ClusterState.CFStackNodeGroupKeyPairName)
}
