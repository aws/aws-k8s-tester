package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-k8s-tester/version"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_kms_v2 "github.com/aws/aws-sdk-go-v2/service/kms"
	aws_kms_v2_types "github.com/aws/aws-sdk-go-v2/service/kms/types"
	smithy "github.com/aws/smithy-go"
	"go.uber.org/zap"
)

func (ts *tester) createEncryption() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_green]createEncryption [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if !ts.cfg.EKSConfig.Encryption.CMKCreate {
		ts.cfg.Logger.Info("Encryption.CMKCreate false; no need to create a new one")
		return nil
	}

	if ts.cfg.EKSConfig.Encryption.CMKARN != "" {
		ts.cfg.Logger.Info("describing the key", zap.String("cmk-arn", ts.cfg.EKSConfig.Encryption.CMKARN))
		_, err := ts.cfg.KMSAPIV2.DescribeKey(
			context.Background(),
			&aws_kms_v2.DescribeKeyInput{
				KeyId: aws_v2.String(ts.cfg.EKSConfig.Encryption.CMKARN),
			})
		if err != nil {
			var apiErr smithy.APIError
			deleted := false
			if errors.As(err, &apiErr) {
				switch apiErr.ErrorCode() {
				case "KMSInvalidStateException":
					deleted = strings.HasSuffix(apiErr.ErrorMessage(), "pending deletion.")
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

		keyARN := ts.cfg.EKSConfig.Encryption.CMKARN
		keyID := getIDFromKeyARN(keyARN)
		ts.cfg.Logger.Info("described the key",
			zap.String("cmk-arn", keyARN),
			zap.String("cmk-id", keyID),
		)
		ts.cfg.EKSConfig.Sync()
		return nil
	}

	ts.cfg.Logger.Info("creating a new KMS CMK")
	out, err := ts.cfg.KMSAPIV2.CreateKey(
		context.Background(),
		&aws_kms_v2.CreateKeyInput{
			Description: aws_v2.String(ts.cfg.EKSConfig.Name),
			Tags: []aws_kms_v2_types.Tag{
				{TagKey: aws_v2.String("Kind"), TagValue: aws_v2.String("aws-k8s-tester")},
				{TagKey: aws_v2.String("Version"), TagValue: aws_v2.String(version.ReleaseVersion)},
				{TagKey: aws_v2.String("User"), TagValue: aws_v2.String(user.Get())},
			},
		})
	if err != nil {
		ts.cfg.Logger.Warn("failed to create a new KMS CMK", zap.Error(err))
		return err
	}

	ts.cfg.EKSConfig.Encryption.CMKARN = aws_v2.ToString(out.KeyMetadata.Arn)
	keyARN := ts.cfg.EKSConfig.Encryption.CMKARN
	keyID := getIDFromKeyARN(keyARN)
	ts.cfg.Logger.Info("created a new KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) deleteEncryption() error {
	fmt.Print(ts.cfg.EKSConfig.Colorize("\n\n[yellow]*********************************\n"))
	fmt.Printf(ts.cfg.EKSConfig.Colorize("[light_blue]deleteEncryption [default](%q)\n"), ts.cfg.EKSConfig.ConfigPath)

	if !ts.cfg.EKSConfig.Encryption.CMKCreate {
		ts.cfg.Logger.Info("Encryption.CMKCreate false; no need to delete one")
		return nil
	}
	if ts.cfg.EKSConfig.Encryption.CMKARN == "" {
		ts.cfg.Logger.Info("Encryption.CMKARN empty; no need to delete one")
		return nil
	}
	if _, ok := ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.Encryption.CMKARN]; ok {
		ts.cfg.Logger.Info("Encryption.CMKARN already deleted; no need to delete one")
		return nil
	}

	keyARN := ts.cfg.EKSConfig.Encryption.CMKARN
	keyID := getIDFromKeyARN(keyARN)

	ts.cfg.Logger.Info("deleting KMS CMK",
		zap.String("cmk-arn", keyARN),
		zap.String("cmk-id", keyID),
	)
	dresp, err := ts.cfg.KMSAPIV2.ScheduleKeyDeletion(
		context.Background(),
		&aws_kms_v2.ScheduleKeyDeletionInput{
			KeyId:               aws_v2.String(keyID),
			PendingWindowInDays: aws_v2.Int32(7),
		})
	if err != nil {
		var apiErr smithy.APIError
		deleted := false
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "KMSInvalidStateException":
				deleted = strings.HasSuffix(apiErr.ErrorMessage(), "pending deletion.")
			case "NotFoundException":
				deleted = true
			}
		}
		if !deleted {
			ts.cfg.Logger.Warn("failed to schedule key deletion", zap.Error(err))
			return err
		}
		ts.cfg.Logger.Info("key already has been deleted or scheduled to delete", zap.Error(err))
		ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.Encryption.CMKARN] = "Encryption.CMKARN"
		ts.cfg.EKSConfig.Sync()
		return nil
	}

	ts.cfg.Logger.Info("scheduled to delete", zap.String("deletion-date", aws_v2.ToTime(dresp.DeletionDate).String()))
	ts.cfg.EKSConfig.Status.DeletedResources[ts.cfg.EKSConfig.Encryption.CMKARN] = "Encryption.CMKARN"
	ts.cfg.EKSConfig.Sync()
	return nil
}

// get "330e3b1a-61c4-4be6-93e0-244180c9f169" from "arn:aws:kms:us-west-2:123:key/330e3b1a-61c4-4be6-93e0-244180c9f169"
func getIDFromKeyARN(arn string) string {
	if ss := strings.Split(arn, "/"); len(ss) > 0 {
		arn = ss[len(ss)-1]
	}
	return arn
}
