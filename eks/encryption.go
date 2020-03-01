package eks

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kms"
	"go.uber.org/zap"
)

func (ts *Tester) createEncryption() error {
	if !ts.cfg.Parameters.EncryptionCMKCreate {
		ts.lg.Info("Parameters.EncryptionCMKCreate false; no need to create a new one")
		return nil
	}

	if ts.cfg.Parameters.EncryptionCMKARN != "" {
		ts.lg.Info("describing the key", zap.String("cmk-arn", ts.cfg.Parameters.EncryptionCMKARN))
		_, err := ts.kmsAPI.DescribeKey(&kms.DescribeKeyInput{
			KeyId: aws.String(ts.cfg.Parameters.EncryptionCMKARN),
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
				ts.lg.Warn("CMK not found", zap.Error(err))
			} else {
				ts.lg.Warn("failed to describe CMK ARN", zap.Error(err))
			}
			return err
		}

		keyARN := ts.cfg.Parameters.EncryptionCMKARN
		keyID := getIDFromKeyARN(keyARN)
		ts.lg.Info("described the key",
			zap.String("cmk-arn", keyARN),
			zap.String("cmk-id", keyID),
		)
		return ts.cfg.Sync()
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

	ts.cfg.Parameters.EncryptionCMKARN = aws.StringValue(out.KeyMetadata.Arn)
	keyARN := ts.cfg.Parameters.EncryptionCMKARN
	keyID := getIDFromKeyARN(keyARN)
	ts.lg.Info("created a new KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	return ts.cfg.Sync()
}

func (ts *Tester) deleteEncryption() error {
	if !ts.cfg.Parameters.EncryptionCMKCreate {
		ts.lg.Info("Parameters.EncryptionCMKCreate false; no need to delete one")
		return nil
	}

	keyARN := ts.cfg.Parameters.EncryptionCMKARN
	keyID := getIDFromKeyARN(keyARN)

	ts.lg.Info("deleting KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	dresp, err := ts.kmsAPI.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
		KeyId:               aws.String(keyID),
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

	ts.lg.Info("scheduled to delete", zap.String("deletion-date", aws.TimeValue(dresp.DeletionDate).String()))
	return ts.cfg.Sync()
}

// get "330e3b1a-61c4-4be6-93e0-244180c9f169" from "arn:aws:kms:us-west-2:123:key/330e3b1a-61c4-4be6-93e0-244180c9f169"
func getIDFromKeyARN(arn string) string {
	if ss := strings.Split(arn, "/"); len(ss) > 0 {
		arn = ss[len(ss)-1]
	}
	return arn
}
