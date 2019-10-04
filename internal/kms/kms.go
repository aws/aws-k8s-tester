// Package kms implements KMS operations.
package kms

import (
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/kmsconfig"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/kms/kmsiface"
	"go.uber.org/zap"
)

// Deployer defines KMS deployer.
type Deployer interface {
	// CreateKey creates a new AWS KMS customer master key (CMK)
	// https://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html
	CreateKey() error
	// ScheduleKeyDeletion schedules a key deletion.
	// Minimum pending days are 7.
	ScheduleKeyDeletion(pendingDays int64) error
	// ListAllKeys lists all KMS keys.
	ListAllKeys() ([]kms.KeyListEntry, error)
}

type deployer struct {
	cfg *kmsconfig.Config

	lg  *zap.Logger
	ss  *session.Session
	kms kmsiface.KMSAPI

	keyMu sync.RWMutex
}

// New returns a new KMS deployer.
func New(cfg *kmsconfig.Config) (Deployer, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lcfg := logutil.AddOutputPaths(logutil.DefaultZapLoggerConfig, cfg.LogOutputs, cfg.LogOutputs)
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(cfg.LogLevel))
	lg, err := lcfg.Build()
	if err != nil {
		return nil, err
	}

	awsCfg := &awsapi.Config{
		Logger:        lg,
		DebugAPICalls: cfg.LogLevel == "debug",
		Region:        cfg.AWSRegion,
	}
	ss, stsOutput, _, err := awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}
	cfg.AWSAccountID = *stsOutput.Account
	cfg.Sync()

	lg.Info("creating a new KMS deployer",
		zap.String("path", cfg.ConfigPath),
	)

	return &deployer{
		cfg: cfg,
		lg:  lg,
		ss:  ss,
		kms: kms.New(ss),
	}, nil
}

func (dp *deployer) CreateKey() error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	ko, err := dp.kms.CreateKey(&kms.CreateKeyInput{
		Description: aws.String(dp.cfg.ID),
		Tags: []*kms.Tag{
			{TagKey: aws.String("Kind"), TagValue: aws.String("aws-k8s-tester")},
		},
	})
	if err != nil {
		return err
	}
	dp.cfg.KeyMetadata = &kmsconfig.KeyMetadata{
		ARN:   aws.StringValue(ko.KeyMetadata.Arn),
		KeyID: aws.StringValue(ko.KeyMetadata.KeyId),
	}

	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	if err = dp.cfg.Sync(); err != nil {
		return err
	}
	dp.lg.Info("created a key",
		zap.String("key-arn", dp.cfg.KeyMetadata.ARN),
		zap.String("key-id", dp.cfg.KeyMetadata.KeyID),
	)
	return nil
}

func (dp *deployer) syncKeyMetadata() (err error) {
	k, err := dp.kms.DescribeKey(&kms.DescribeKeyInput{
		KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
	})
	if err != nil {
		return err
	}
	dp.cfg.KeyMetadata = &kmsconfig.KeyMetadata{
		AWSAccountID: aws.StringValue(k.KeyMetadata.AWSAccountId),
		ARN:          aws.StringValue(k.KeyMetadata.Arn),
		CreationDate: aws.TimeValue(k.KeyMetadata.CreationDate),
		Description:  aws.StringValue(k.KeyMetadata.Description),
		Enabled:      aws.BoolValue(k.KeyMetadata.Enabled),
		KeyID:        aws.StringValue(k.KeyMetadata.KeyId),
		KeyManager:   aws.StringValue(k.KeyMetadata.KeyManager),
		KeyState:     aws.StringValue(k.KeyMetadata.KeyState),
		KeyUsage:     aws.StringValue(k.KeyMetadata.KeyUsage),
		Origin:       aws.StringValue(k.KeyMetadata.Origin),
	}
	return nil
}

func (dp *deployer) ScheduleKeyDeletion(pendingDays int64) error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	dresp, err := dp.kms.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(dp.cfg.KeyMetadata.KeyID),
		PendingWindowInDays: aws.Int64(pendingDays),
	})
	if err != nil {
		dp.lg.Warn("failed to schedule key deletion", zap.Error(err))
		return err
	}
	dp.lg.Warn("scheduled to delete",
		zap.String("deletion-date", aws.TimeValue(dresp.DeletionDate).String()),
		zap.String("key-id", dp.cfg.KeyMetadata.KeyID),
		zap.String("key-arn", dp.cfg.KeyMetadata.ARN),
	)

	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	return dp.cfg.Sync()
}

func (dp *deployer) ListAllKeys() (entries []kms.KeyListEntry, err error) {
	entries = make([]kms.KeyListEntry, 0)
	waitTime := 7 * time.Minute
	retryStart := time.Now().UTC()

	input := &kms.ListKeysInput{}
	for time.Now().UTC().Sub(retryStart) < waitTime {
		var listOutput *kms.ListKeysOutput
		listOutput, err = dp.kms.ListKeys(input)
		if err != nil {
			if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
				dp.lg.Warn("retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			awsErr, ok := err.(awserr.Error)
			if ok {
				dp.lg.Warn("failed to list keys", zap.String("code", awsErr.Code()), zap.Error(err))
			}
			return nil, err
		}

		for _, v := range listOutput.Keys {
			entries = append(entries, *v)
		}

		if aws.StringValue(listOutput.NextMarker) == "" {
			break
		}
		input.Marker = listOutput.NextMarker
	}

	dp.lg.Info("listed all KMS keys", zap.Int("keys", len(entries)))
	return entries, nil
}
