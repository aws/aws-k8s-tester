package eks

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kms"
	"go.uber.org/zap"
)

func (ts *Tester) createEncryption() error {
	if ts.cfg.Parameters.EncryptionCMKARN != "" &&
		(ts.cfg.Status.EncryptionCMKARN == "" || ts.cfg.Status.EncryptionCMKID == "") {
		ts.lg.Info("describing the key", zap.String("cmk-arn", ts.cfg.Parameters.EncryptionCMKARN))
		dresp, err := ts.kmsAPI.DescribeKey(&kms.DescribeKeyInput{
			KeyId: aws.String(ts.cfg.Parameters.EncryptionCMKARN),
		})
		if err != nil {
			ts.lg.Warn("failed to describe CMK ARN", zap.Error(err))
			return err
		}
		ts.cfg.Status.EncryptionCMKARN = aws.StringValue(dresp.KeyMetadata.Arn)
		ts.cfg.Status.EncryptionCMKID = aws.StringValue(dresp.KeyMetadata.KeyId)
		ts.lg.Info("described the key",
			zap.String("cmk-arn", ts.cfg.Status.EncryptionCMKARN),
			zap.String("cmk-id", ts.cfg.Status.EncryptionCMKID),
		)
		return ts.cfg.Sync()
	}
	if !ts.cfg.Parameters.EncryptionCMKCreate {
		ts.lg.Info("Parameters.EncryptionCMKCreate false; no need to create a new one")
		return nil
	}
	if ts.cfg.Status.EncryptionCMKARN != "" {
		ts.lg.Info("Status.EncryptionCMKARN non-empty; no need to create a new one")
		return nil
	}
	if ts.cfg.Status.EncryptionCMKID != "" {
		ts.lg.Info("Status.EncryptionCMKID non-empty; no need to create a new one")
		return nil
	}

	ts.lg.Info("creating a new KMS CMK")
	out, err := ts.kmsAPI.CreateKey(&kms.CreateKeyInput{
		Description: aws.String(ts.cfg.Name),
		Tags: []*kms.Tag{
			{TagKey: aws.String("Kind"), TagValue: aws.String("aws-k8s-tester")},
		},
	})
	if err != nil {
		ts.lg.Warn("failed to create a new KMS CMK", zap.Error(err))
		return err
	}

	ts.cfg.Status.EncryptionCMKARN = aws.StringValue(out.KeyMetadata.Arn)
	ts.cfg.Status.EncryptionCMKID = aws.StringValue(out.KeyMetadata.KeyId)
	ts.lg.Info("created a new KMS CMK",
		zap.String("cmk-arn", ts.cfg.Status.EncryptionCMKARN),
		zap.String("cmk-id", ts.cfg.Status.EncryptionCMKID),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteEncryption() error {
	if !ts.cfg.Parameters.EncryptionCMKCreate {
		ts.lg.Info("Parameters.EncryptionCMKCreate false; no need to delete one")
		return nil
	}

	ts.lg.Info("deleting KMS CMK", zap.String("cmk-id", ts.cfg.Status.EncryptionCMKID))
	dresp, err := ts.kmsAPI.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(ts.cfg.Status.EncryptionCMKID),
		PendingWindowInDays: aws.Int64(7),
	})
	if err != nil {
		aerr, ok := err.(awserr.Error)
		deleted := false
		if ok {
			switch aerr.Code() {
			case "KMSInvalidStateException":
				deleted = strings.HasSuffix(aerr.Message(), "pending deletion.")
			case "NotFoundException":
				deleted = true
			}
		}
		if !deleted {
			ts.lg.Warn("failed to schedule key deletion", zap.Error(err))
			return err
		}
		ts.lg.Info("key already has been deleted or scheduled to delete", zap.Error(err))
		return ts.cfg.Sync()
	}

	ts.lg.Info("scheduled to delete",
		zap.String("deletion-date", aws.TimeValue(dresp.DeletionDate).String()),
		zap.String("cmk-id", ts.cfg.Status.EncryptionCMKID),
	)
	return ts.cfg.Sync()
}
