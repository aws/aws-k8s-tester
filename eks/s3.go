package eks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

func (ts *Tester) createS3() (err error) {
	if ts.cfg.S3BucketCreate {
		if ts.cfg.S3BucketName == "" {
			return errors.New("empty S3 bucket name")
		}
		var retry bool
		for i := 0; i < 5; i++ {
			retry, err = ts.createBucket()
			if err == nil {
				break
			}
			if retry {
				ts.lg.Warn("failed to create bucket; retrying", zap.Error(err))
				time.Sleep(5 * time.Second)
				continue
			}
			return err
		}
		if err != nil {
			return err
		}
	} else {
		ts.lg.Info("skipping S3 bucket creation")
	}
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 bucket creation")
		return nil
	}

	testKey := path.Join(ts.cfg.Name, "test-"+getTS()+".log")
	_, err = ts.s3API.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Key:    aws.String(testKey),
		Body:   strings.NewReader("Hello World!"),

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
		},
	})
	if err == nil {
		ts.lg.Info("uploaded",
			zap.String("bucket", ts.cfg.S3BucketName),
			zap.String("remote-path", testKey),
		)
	} else {
		ts.lg.Warn("failed to upload",
			zap.String("bucket", ts.cfg.S3BucketName),
			zap.String("remote-path", testKey),
			zap.Error(err),
		)
	}
	return ts.cfg.Sync()
}

func (ts *Tester) createBucket() (bool, error) {
	if !ts.cfg.S3BucketCreate {
		ts.lg.Info("skipping S3 bucket creation", zap.String("name", ts.cfg.S3BucketName))
		return false, nil
	}
	ts.lg.Info("creating S3 bucket", zap.String("name", ts.cfg.S3BucketName))
	_, err := ts.s3API.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(ts.cfg.Region),
		},
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),
	})
	alreadyExist := false
	if err != nil {
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				ts.lg.Warn("bucket already exists", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				alreadyExist, err = true, nil
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				ts.lg.Warn("bucket already owned by me", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				alreadyExist, err = true, nil
			default:
				if strings.Contains(err.Error(), "OperationAborted: A conflicting conditional operation is currently in progress against this resource. Please try again.") ||
					request.IsErrorRetryable(err) ||
					request.IsErrorThrottle(err) {
					return true, err
				}
				ts.lg.Warn("failed to create bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.String("code", aerr.Code()), zap.Error(err))
				return false, err
			}
		}
		if !alreadyExist {
			ts.lg.Warn("failed to create bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.String("type", reflect.TypeOf(err).String()), zap.Error(err))
			return false, err
		}
	}
	if alreadyExist {
		return false, nil
	}
	ts.lg.Info("created S3 bucket", zap.String("bucket", ts.cfg.S3BucketName))

	_, err = ts.s3API.PutBucketTagging(&s3.PutBucketTaggingInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Tagging: &s3.Tagging{TagSet: []*s3.Tag{
			{Key: aws.String("Kind"), Value: aws.String("aws-k8s-tester")},
			{Key: aws.String("Creation"), Value: aws.String(time.Now().String())},
		}},
	})
	if err != nil {
		return true, err
	}

	if ts.cfg.S3BucketLifecycleExpirationDays > 0 {
		_, err = ts.s3API.PutBucketLifecycle(&s3.PutBucketLifecycleInput{
			Bucket: aws.String(ts.cfg.S3BucketName),
			LifecycleConfiguration: &s3.LifecycleConfiguration{
				Rules: []*s3.Rule{
					{
						Prefix: aws.String(ts.cfg.Name),
						AbortIncompleteMultipartUpload: &s3.AbortIncompleteMultipartUpload{
							DaysAfterInitiation: aws.Int64(ts.cfg.S3BucketLifecycleExpirationDays),
						},
						Expiration: &s3.LifecycleExpiration{
							Days: aws.Int64(ts.cfg.S3BucketLifecycleExpirationDays),
						},
						ID:     aws.String(fmt.Sprintf("ObjectLifecycleOf%vDays", ts.cfg.S3BucketLifecycleExpirationDays)),
						Status: aws.String("Enabled"),
					},
				},
			},
		})
		if err != nil {
			return true, err
		}
	}

	return false, ts.cfg.Sync()
}

func (ts *Tester) deleteS3() error {
	if !ts.cfg.S3BucketCreate {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName))
		return nil
	}
	if ts.cfg.S3BucketCreateKeep {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName), zap.Bool("s3-bucket-create-keep", ts.cfg.S3BucketCreateKeep))
		return nil
	}

	ts.lg.Info("emptying bucket", zap.String("bucket", ts.cfg.S3BucketName))
	batcher := s3manager.NewBatchDeleteWithClient(ts.s3API)
	iter := &s3manager.DeleteListIterator{
		Bucket: aws.String(ts.cfg.S3BucketName),
		Paginator: request.Pagination{
			NewRequest: func() (*request.Request, error) {
				req, _ := ts.s3API.ListObjectsRequest(&s3.ListObjectsInput{
					Bucket: aws.String(ts.cfg.S3BucketName),
				})
				return req, nil
			},
		},
	}
	err := batcher.Delete(aws.BackgroundContext(), iter)
	if err != nil { // https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				ts.lg.Info("no such bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				return nil
			}
		}
		ts.lg.Warn("failed to empty bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
		return err
	}
	ts.lg.Info("emptied bucket", zap.String("bucket", ts.cfg.S3BucketName))

	ts.lg.Info("deleting bucket", zap.String("bucket", ts.cfg.S3BucketName))
	_, err = ts.s3API.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(ts.cfg.S3BucketName),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				ts.lg.Info("no such bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
				return nil
			}
		}
		ts.lg.Warn("failed to delete bucket", zap.String("bucket", ts.cfg.S3BucketName), zap.Error(err))
	}

	ts.lg.Info("deleted bucket", zap.String("bucket", ts.cfg.S3BucketName))
	return nil
}

func getTS() string {
	now := time.Now()
	return fmt.Sprintf(
		"%04d%02d%02d%02d%02d",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		now.Second(),
	)
}

func (ts *Tester) uploadToS3() (err error) {
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 uploads; s3 bucket name is empty")
		return nil
	}

	if err = uploadFileToS3(
		ts.lg,
		ts.s3API,
		ts.cfg.S3BucketName,
		path.Join(ts.cfg.Name, "aws-k8s-tester-eks.config.yaml"),
		ts.cfg.ConfigPath,
	); err != nil {
		return err
	}

	logFilePath := ""
	for _, fpath := range ts.cfg.LogOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if fileutil.Exist(logFilePath) {
		if err = uploadFileToS3(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "aws-k8s-tester-eks.log"),
			logFilePath,
		); err != nil {
			return err
		}
	}

	if err = uploadFileToS3(
		ts.lg,
		ts.s3API,
		ts.cfg.S3BucketName,
		path.Join(ts.cfg.Name, "kubeconfig.yaml"),
		ts.cfg.KubeConfigPath,
	); err != nil {
		return err
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if fileutil.Exist(ts.cfg.AddOnNodeGroups.LogsTarGzPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "node-groups-logs-dir.tar.gz"),
				ts.cfg.AddOnConformance.SonobuoyResultTarGzPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		if fileutil.Exist(ts.cfg.AddOnManagedNodeGroups.LogsTarGzPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "managed-node-groups-logs-dir.tar.gz"),
				ts.cfg.AddOnConformance.SonobuoyResultTarGzPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConformance() {
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultTarGzPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.tar.gz"),
				ts.cfg.AddOnConformance.SonobuoyResultTarGzPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultE2eLogPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.e2e.log"),
				ts.cfg.AddOnConformance.SonobuoyResultE2eLogPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultJunitXMLPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.junit.xml"),
				ts.cfg.AddOnConformance.SonobuoyResultJunitXMLPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnCSRsLocal() {
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes.json"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes.txt"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnCSRsRemote() {
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes.json"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigMapsLocal() {
		if fileutil.Exist(ts.cfg.AddOnConfigMapsLocal.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "config-maps-local-requests-summary-writes.json"),
				ts.cfg.AddOnConfigMapsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigMapsLocal.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "config-maps-local-requests-summary-writes.txt"),
				ts.cfg.AddOnConfigMapsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigMapsRemote() {
		if fileutil.Exist(ts.cfg.AddOnConfigMapsRemote.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "config-maps-remote-requests-summary-writes.json"),
				ts.cfg.AddOnConfigMapsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigMapsRemote.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "config-maps-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnConfigMapsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsLocal() {
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes.json"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsRemote() {
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnStresserLocal() {
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnStresserRemote() {
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnClusterLoaderLocal() {
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderLocal.ClusterLoaderLogsPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-local-logs.log"),
				ts.cfg.AddOnClusterLoaderLocal.ClusterLoaderLogsPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnClusterLoaderRemote() {
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderRemote.ClusterLoaderLogsPath) {
			if err = uploadFileToS3(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-remote-logs.log"),
				ts.cfg.AddOnClusterLoaderRemote.ClusterLoaderLogsPath,
			); err != nil {
				return err
			}
		}
	}

	return err
}

func uploadFileToS3(lg *zap.Logger, s3API s3iface.S3API, bucketName string, s3Key string, fpath string) error {
	stat, err := os.Stat(fpath)
	if err != nil {
		return err
	}
	size := humanize.Bytes(uint64(stat.Size()))

	rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		ts.cfg.Logger.Warn("failed to read a file", zap.String("file-path", fpath), zap.Error(err))
		return err
	}
	defer rf.Close()

	lg.Info("uploading",
		zap.String("bucket", bucketName),
		zap.String("remote-path", s3Key),
		zap.String("file-size", size),
	)
	_, err = s3API.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(s3Key),
		Body:   rf,

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
		},
	})
	if err == nil {
		lg.Info("uploaded",
			zap.String("bucket", bucketName),
			zap.String("remote-path", s3Key),
			zap.String("file-size", size),
		)
	} else {
		lg.Warn("failed to upload",
			zap.String("bucket", bucketName),
			zap.String("remote-path", s3Key),
			zap.String("file-size", size),
			zap.Error(err),
		)
	}
	return err
}
