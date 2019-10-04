// Package kms implements KMS operations.
package kms

import (
	"fmt"
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

	EnableKey() error
	DisableKey() error

	EnableKeyRotation() error
	DisableKeyRotation() error

	GenerateDataKey(encryptionCtx map[string]string) (cipher []byte, plain []byte, err error)
	Encrypt(encryptionCtx map[string]string, plain []byte) (cipher []byte, err error)
	Decrypt(encryptionCtx map[string]string, cipher []byte) (plain []byte, err error)
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

	lg.Info("creating a new KMS deployer", zap.String("path", cfg.ConfigPath))

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
	if !dp.cfg.KeyMetadata.Enabled {
		return fmt.Errorf("KeyMetadata.Enabled unexpected %v", dp.cfg.KeyMetadata.Enabled)
	}
	dp.lg.Info("created a key",
		zap.String("key-arn", dp.cfg.KeyMetadata.ARN),
		zap.String("key-id", dp.cfg.KeyMetadata.KeyID),
	)
	return nil
}

func (dp *deployer) syncKeyMetadata() (err error) {
	desc := &kms.DescribeKeyOutput{}
	rt := &kms.GetKeyRotationStatusOutput{}

	waitTime := 7 * time.Minute
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < waitTime {
		var err error
		desc, err = dp.kms.DescribeKey(&kms.DescribeKeyInput{
			KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
		})
		if err != nil {
			if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
				dp.lg.Warn("retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			awsErr, ok := err.(awserr.Error)
			if ok {
				dp.lg.Warn("failed to describe key", zap.String("code", awsErr.Code()), zap.Error(err))
			}
			return err
		}

		rt, err = dp.kms.GetKeyRotationStatus(&kms.GetKeyRotationStatusInput{
			KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
		})
		if err != nil {
			if request.IsErrorRetryable(err) || request.IsErrorThrottle(err) {
				dp.lg.Warn("retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			awsErr, ok := err.(awserr.Error)
			if ok {
				dp.lg.Warn("failed to describe key", zap.String("code", awsErr.Code()), zap.Error(err))
			}
			return err
		}

		break
	}

	dp.cfg.KeyMetadata = &kmsconfig.KeyMetadata{
		AWSAccountID: aws.StringValue(desc.KeyMetadata.AWSAccountId),
		ARN:          aws.StringValue(desc.KeyMetadata.Arn),
		CreationDate: aws.TimeValue(desc.KeyMetadata.CreationDate),
		Description:  aws.StringValue(desc.KeyMetadata.Description),
		Enabled:      aws.BoolValue(desc.KeyMetadata.Enabled),
		KeyID:        aws.StringValue(desc.KeyMetadata.KeyId),
		KeyManager:   aws.StringValue(desc.KeyMetadata.KeyManager),
		KeyState:     aws.StringValue(desc.KeyMetadata.KeyState),
		KeyUsage:     aws.StringValue(desc.KeyMetadata.KeyUsage),
		Origin:       aws.StringValue(desc.KeyMetadata.Origin),
	}
	dp.cfg.KeyRotationEnabled = aws.BoolValue(rt.KeyRotationEnabled)

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
	if dp.cfg.KeyMetadata.KeyState != "PendingDeletion" {
		return fmt.Errorf("KeyMetadata.KeyState unexpected %q", dp.cfg.KeyMetadata.KeyState)
	}
	return dp.cfg.Sync()
}

func (dp *deployer) ListAllKeys() (entries []kms.KeyListEntry, err error) {
	entries = make([]kms.KeyListEntry, 0)

	input := &kms.ListKeysInput{}

	waitTime := 7 * time.Minute
	retryStart := time.Now().UTC()
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
		time.Sleep(5 * time.Second)
	}

	dp.lg.Info("listed all KMS keys", zap.Int("keys", len(entries)))
	return entries, nil
}

func (dp *deployer) EnableKey() error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	_, err := dp.kms.EnableKey(&kms.EnableKeyInput{
		KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
	})
	if err != nil {
		return err
	}
	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	if !dp.cfg.KeyMetadata.Enabled {
		return fmt.Errorf("%q expected Enabled true, got %v", dp.cfg.KeyMetadata.KeyID, dp.cfg.KeyMetadata.Enabled)
	}
	return nil
}

func (dp *deployer) DisableKey() error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	_, err := dp.kms.DisableKey(&kms.DisableKeyInput{
		KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
	})
	if err != nil {
		return err
	}
	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	if dp.cfg.KeyMetadata.Enabled {
		return fmt.Errorf("%q expected Enabled false, got %v", dp.cfg.KeyMetadata.KeyID, dp.cfg.KeyMetadata.Enabled)
	}
	return nil
}

func (dp *deployer) EnableKeyRotation() error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	_, err := dp.kms.EnableKeyRotation(&kms.EnableKeyRotationInput{
		KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
	})
	if err != nil {
		return err
	}
	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	if !dp.cfg.KeyRotationEnabled {
		return fmt.Errorf("%q expected KeyRotationEnabled false, got %v", dp.cfg.KeyMetadata.KeyID, dp.cfg.KeyRotationEnabled)
	}
	return nil
}

func (dp *deployer) DisableKeyRotation() error {
	dp.keyMu.Lock()
	defer dp.keyMu.Unlock()

	_, err := dp.kms.DisableKeyRotation(&kms.DisableKeyRotationInput{
		KeyId: aws.String(dp.cfg.KeyMetadata.KeyID),
	})
	if err != nil {
		return err
	}
	if err = dp.syncKeyMetadata(); err != nil {
		return err
	}
	if dp.cfg.KeyRotationEnabled {
		return fmt.Errorf("%q expected KeyRotationEnabled false, got %v", dp.cfg.KeyMetadata.KeyID, dp.cfg.KeyRotationEnabled)
	}
	return nil
}

func (dp *deployer) GenerateDataKey(encryptionCtx map[string]string, keySpec string, keyBytes int64) (cipher []byte, plain []byte, err error) {
	dp.keyMu.RLock()
	defer dp.keyMu.RUnlock()

	ctx := make(map[string]*string, len(encryptionCtx))
	for k, v := range encryptionCtx {
		ctx[k] = aws.String(v)
	}

	out, err := dp.kms.GenerateDataKey(&kms.GenerateDataKeyInput{
		EncryptionContext: ctx,
		KeyId:             aws.String(dp.cfg.KeyMetadata.KeyID),
		KeySpec:           aws.String(keySpec),
		NumberOfBytes:     aws.Int64(keyBytes),
	})
	if err != nil {
		return nil, nil, err
	}
	if dp.cfg.KeyMetadata.KeyID != aws.StringValue(out.KeyId) {
		return nil, nil, fmt.Errorf("expected key ID %q, got %q", dp.cfg.KeyMetadata.KeyID, aws.StringValue(out.KeyId))
	}
	if err = dp.syncKeyMetadata(); err != nil {
		return nil, nil, err
	}
	return out.CiphertextBlob, out.Plaintext, nil
}

func (dp *deployer) Encrypt(encryptionCtx map[string]string, plain []byte) (cipher []byte, err error) {
	dp.keyMu.RLock()
	defer dp.keyMu.RUnlock()

	ctx := make(map[string]*string, len(encryptionCtx))
	for k, v := range encryptionCtx {
		ctx[k] = aws.String(v)
	}

	var resp *kms.EncryptOutput
	resp, err = dp.kms.Encrypt(&kms.EncryptInput{
		EncryptionContext: ctx,
		KeyId:             aws.String(dp.cfg.KeyMetadata.KeyID),
		Plaintext:         plain,
	})
	if err != nil {
		dp.lg.Warn("failed to encrypt", zap.Error(err))
		return nil, err
	}
	return resp.CiphertextBlob, nil
}

func (dp *deployer) Decrypt(encryptionCtx map[string]string, cipher []byte) (plain []byte, err error) {
	dp.keyMu.RLock()
	defer dp.keyMu.RUnlock()

	ctx := make(map[string]*string, len(encryptionCtx))
	for k, v := range encryptionCtx {
		ctx[k] = aws.String(v)
	}

	var resp *kms.DecryptOutput
	resp, err = dp.kms.Decrypt(&kms.DecryptInput{
		EncryptionContext: ctx,
		CiphertextBlob:    cipher,
	})
	if err != nil {
		dp.lg.Warn("failed to decrypt", zap.Error(err))
		return nil, err
	}
	return resp.Plaintext, nil
}
