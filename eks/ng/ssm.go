package ng

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	aws_v2 "github.com/aws/aws-sdk-go-v2/aws"
	aws_ssm_v2 "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (ts *tester) createSSM() error {
	if err := ts.createSSMDocument(); err != nil {
		return err
	}
	if err := ts.sendSSMDocumentCommand(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) deleteSSM() error {
	if err := ts.deleteSSMDocument(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) createSSMDocument() error {
	createStart := time.Now()

	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if cur.SSM == nil {
			continue
		}

		if !cur.SSM.DocumentCreate {
			ts.cfg.Logger.Info("skipping SSM document create",
				zap.String("asg-name", asgName),
				zap.String("ssm-document-name", cur.SSM.DocumentName),
			)
			continue
		}

		ts.cfg.Logger.Info("creating SSM document",
			zap.String("asg-name", asgName),
			zap.String("ssm-document-name", cur.SSM.DocumentName),
		)

		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()

		ts.cfg.Logger.Info("created SSM Document",
			zap.String("asg-name", cur.Name),
			zap.String("ssm-document-name", cur.SSM.DocumentName),
			zap.String("started", humanize.RelTime(createStart, time.Now(), "ago", "from now")),
		)
		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) deleteSSMDocument() error {
	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if cur.SSM == nil {
			continue
		}

		if !cur.SSM.DocumentCreate {
			ts.cfg.Logger.Info("skipping SSM document delete",
				zap.String("asg-name", asgName),
				zap.String("ssm-document-name", cur.SSM.DocumentName),
			)
			continue
		}
		ts.cfg.Logger.Info("deleting SSM document",
			zap.String("asg-name", cur.Name),
			zap.String("ssm-document-name", cur.SSM.DocumentName),
		)

		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("%q/%s", cur.SSM.DocumentName, ec2config.StatusDELETEDORNOTEXIST))

		ts.cfg.Logger.Info("deleted SSM document",
			zap.String("asg-name", cur.Name),
			zap.String("ssm-document-name", cur.SSM.DocumentName),
		)
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) sendSSMDocumentCommand() error {
	for asgName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if cur.SSM == nil {
			continue
		}

		if cur.SSM.DocumentName == "" {
			ts.cfg.Logger.Info("skipping SSM document send",
				zap.String("asg-name", asgName),
				zap.String("ssm-document-name", cur.SSM.DocumentName),
			)
			continue
		}
		if len(cur.Instances) == 0 {
			return fmt.Errorf("no instance found for SSM document %q", cur.SSM.DocumentName)
		}
		ids := make([]string, 0)
		for id := range cur.Instances {
			ids = append(ids, id)
		}

		// batch by 50
		// e.g. 'instanceIds' failed to satisfy constraint: Member must have length less than or equal to 50
		ts.cfg.Logger.Info("sending SSM document",
			zap.String("asg-name", asgName),
			zap.String("ssm-document-name", cur.SSM.DocumentName),
			zap.Int("instance-ids", len(ids)),
		)
		left := make([]string, len(ids))
		copy(left, ids)
		for len(left) > 0 {
			batch := make([]string, 0)
			switch {
			case len(left) <= 50:
				batch = append(batch, left...)
				left = left[:0:0]
			case len(left) > 50:
				batch = append(batch, left[:50]...)
				left = left[50:]
			}
			ssmInput := &aws_ssm_v2.SendCommandInput{
				DocumentName:   aws_v2.String(cur.SSM.DocumentName),
				Comment:        aws_v2.String(cur.SSM.DocumentName + "-" + randutil.String(10)),
				InstanceIds:    batch,
				MaxConcurrency: aws_v2.String(fmt.Sprintf("%d", len(batch))),
				Parameters: map[string][]string{
					"region":                  {ts.cfg.EKSConfig.Region},
					"executionTimeoutSeconds": {fmt.Sprintf("%d", cur.SSM.DocumentExecutionTimeoutSeconds)},
				},
				OutputS3BucketName: aws_v2.String(ts.cfg.EKSConfig.S3.BucketName),
				OutputS3KeyPrefix:  aws_v2.String(path.Join(ts.cfg.EKSConfig.Name, "ssm-outputs")),
			}
			if len(cur.SSM.DocumentCommands) > 0 {
				ssmInput.Parameters["moreCommands"] = []string{cur.SSM.DocumentCommands}
			}
			cmd, err := ts.cfg.SSMAPIV2.SendCommand(
				context.Background(),
				ssmInput,
			)
			if err != nil {
				return err
			}
			docName := aws_v2.ToString(cmd.Command.DocumentName)
			if docName != cur.SSM.DocumentName {
				return fmt.Errorf("SSM Document Name expected %q, got %q", cur.SSM.DocumentName, docName)
			}
			cmdID := aws_v2.ToString(cmd.Command.CommandId)
			cur.SSM.DocumentCommandIDs = append(cur.SSM.DocumentCommandIDs, cmdID)

			ts.cfg.Logger.Info("sent SSM document",
				zap.String("asg-name", asgName),
				zap.String("ssm-document-name", cur.SSM.DocumentName),
				zap.String("ssm-command-id", cmdID),
				zap.Int("sent-instance-ids", len(batch)),
				zap.Int("left-instance-ids", len(left)),
			)
			if len(left) == 0 {
				break
			}

			ts.cfg.Logger.Info("waiting for next SSM run batch", zap.Int("left", len(left)))
			time.Sleep(15 * time.Second)
		}

		ts.cfg.EKSConfig.AddOnNodeGroups.ASGs[asgName] = cur
		ts.cfg.EKSConfig.Sync()
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}
