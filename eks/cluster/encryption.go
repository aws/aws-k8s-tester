package cluster

import (
	"fmt"
	"strings"

	"github.com/aws/aws-k8s-tester/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/kms"
	"go.uber.org/zap"
)

func (ts *tester) createEncryption() error {
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createEncryption (%q)\n"), ts.cfg.EKSConfig.ConfigPath)
	fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))

	if !ts.cfg.EKSConfig.Parameters.EncryptionCMKCreate {
		ts.cfg.Logger.Info("Parameters.EncryptionCMKCreate false; no need to create a new one")
		return nil
	}

	if ts.cfg.EKSConfig.Parameters.EncryptionCMKARN != "" {
		ts.cfg.Logger.Info("describing the key", zap.String("cmk-arn", ts.cfg.EKSConfig.Parameters.EncryptionCMKARN))
		_, err := ts.cfg.KMSAPI.DescribeKey(&kms.DescribeKeyInput{
			KeyId: aws.String(ts.cfg.EKSConfig.Parameters.EncryptionCMKARN),
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
				ts.cfg.Logger.Warn("CMK not found", zap.Error(err))
			} else {
				ts.cfg.Logger.Warn("failed to describe CMK ARN", zap.Error(err))
			}
			return err
		}

		keyARN := ts.cfg.EKSConfig.Parameters.EncryptionCMKARN
		keyID := getIDFromKeyARN(keyARN)
		ts.cfg.Logger.Info("described the key",
			zap.String("cmk-arn", keyARN),
			zap.String("cmk-id", keyID),
		)
		return ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("creating a new KMS CMK")
	out, err := ts.cfg.KMSAPI.CreateKey(&kms.CreateKeyInput{
		Description: aws.String(ts.cfg.EKSConfig.Name),
		Tags: []*kms.Tag{
			{TagKey: aws.String("Kind"), TagValue: aws.String("aws-k8s-tester")},
			{TagKey: aws.String("Version"), TagValue: aws.String(version.ReleaseVersion)},
		},
	})
	if err != nil {
		ts.cfg.Logger.Warn("failed to create a new KMS CMK", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.Parameters.EncryptionCMKARN = aws.StringValue(out.KeyMetadata.Arn)
	keyARN := ts.cfg.EKSConfig.Parameters.EncryptionCMKARN
	keyID := getIDFromKeyARN(keyARN)
	ts.cfg.Logger.Info("created a new KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteEncryption() error {
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_blue]deleteEncryption (%q)\n"), ts.cfg.EKSConfig.ConfigPath)
	fmt.Printf(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))

	if !ts.cfg.EKSConfig.Parameters.EncryptionCMKCreate {
		ts.cfg.Logger.Info("Parameters.EncryptionCMKCreate false; no need to delete one")
		return nil
	}
	if ts.cfg.EKSConfig.Parameters.EncryptionCMKARN == "" {
		ts.cfg.Logger.Info("Parameters.EncryptionCMKARN empty; no need to delete one")
		return nil
	}

	keyARN := ts.cfg.EKSConfig.Parameters.EncryptionCMKARN
	keyID := getIDFromKeyARN(keyARN)

	ts.cfg.Logger.Info("deleting KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	dresp, err := ts.cfg.KMSAPI.ScheduleKeyDeletion(&kms.ScheduleKeyDeletionInput{
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
			ts.cfg.Logger.Warn("failed to schedule key deletion", zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("key already has been deleted or scheduled to delete", zap.Error(err))
		return ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.Logger.Info("scheduled to delete", zap.String("deletion-date", aws.TimeValue(dresp.DeletionDate).String()))
	return ts.cfg.EKSConfig.Sync()
}

// get "330e3b1a-61c4-4be6-93e0-244180c9f169" from "arn:aws:kms:us-west-2:123:key/330e3b1a-61c4-4be6-93e0-244180c9f169"
func getIDFromKeyARN(arn string) string {
	if ss := strings.Split(arn, "/"); len(ss) > 0 {
		arn = ss[len(ss)-1]
	}
	return arn
}
