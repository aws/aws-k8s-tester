// Package s3 implements S3 utilities.
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/user"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// CreateBucket creates a S3 bucket.
func CreateBucket(
	lg *zap.Logger,
	s3API s3iface.S3API,
	bucket string,
	region string,
	lifecyclePrefix string,
	lifecycleExpirationDays int64) (err error) {

	var retry bool
	for i := 0; i < 5; i++ {
		retry, err = createBucket(lg, s3API, bucket, region, lifecyclePrefix, lifecycleExpirationDays)
		if err == nil {
			break
		}
		if retry {
			lg.Warn("failed to create bucket; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		return err
	}
	return err
}

func createBucket(
	lg *zap.Logger,
	s3API s3iface.S3API,
	bucket string,
	region string,
	lifecyclePrefix string,
	lifecycleExpirationDays int64) (retry bool, err error) {

	lg.Info("creating S3 bucket", zap.String("name", bucket))
	createBucketInput := &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),
	}
	// Setting LocationConstraint to us-east-1 fails with InvalidLocationConstraint. This region is handled differerntly and must be omitted.
	// https://github.com/boto/boto3/issues/125
	if region != "us-east-1" {
		createBucketInput.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(region),
		}
	}
	_, err = s3API.CreateBucket(createBucketInput)
	alreadyExist := false
	if err != nil {
		// https://docs.aws.amazon.com/AWSEC2/latest/APIReference/errors-overview.html
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				lg.Warn("bucket already exists", zap.String("s3-bucket", bucket), zap.Error(err))
				alreadyExist, err = true, nil
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				lg.Warn("bucket already owned by me", zap.String("s3-bucket", bucket), zap.Error(err))
				alreadyExist, err = true, nil
			default:
				if strings.Contains(err.Error(), "OperationAborted: A conflicting conditional operation is currently in progress against this resource. Please try again.") ||
					request.IsErrorRetryable(err) ||
					request.IsErrorThrottle(err) {
					return true, err
				}
				lg.Warn("failed to create bucket", zap.String("s3-bucket", bucket), zap.String("code", aerr.Code()), zap.Error(err))
				return false, err
			}
		}
		if !alreadyExist {
			lg.Warn("failed to create bucket", zap.String("s3-bucket", bucket), zap.String("type", reflect.TypeOf(err).String()), zap.Error(err))
			return false, err
		}
	}
	if alreadyExist {
		return false, nil
	}
	lg.Info("created S3 bucket", zap.String("s3-bucket", bucket))

	_, err = s3API.PutBucketTagging(&s3.PutBucketTaggingInput{
		Bucket: aws.String(bucket),
		Tagging: &s3.Tagging{TagSet: []*s3.Tag{
			{Key: aws.String("Kind"), Value: aws.String("aws-k8s-tester")},
			{Key: aws.String("Creation"), Value: aws.String(time.Now().String())},
		}},
	})
	if err != nil {
		return true, err
	}

	if lifecyclePrefix != "" && lifecycleExpirationDays > 0 {
		_, err = s3API.PutBucketLifecycle(&s3.PutBucketLifecycleInput{
			Bucket: aws.String(bucket),
			LifecycleConfiguration: &s3.LifecycleConfiguration{
				Rules: []*s3.Rule{
					{
						Prefix: aws.String(lifecyclePrefix),
						AbortIncompleteMultipartUpload: &s3.AbortIncompleteMultipartUpload{
							DaysAfterInitiation: aws.Int64(lifecycleExpirationDays),
						},
						Expiration: &s3.LifecycleExpiration{
							Days: aws.Int64(lifecycleExpirationDays),
						},
						ID:     aws.String(fmt.Sprintf("ObjectLifecycleOf%vDays", lifecycleExpirationDays)),
						Status: aws.String("Enabled"),
					},
				},
			},
		})
		if err != nil {
			return true, err
		}
	}

	return false, nil
}

// Upload uploads a file to S3 bucket.
func Upload(
	lg *zap.Logger,
	s3API s3iface.S3API,
	bucket string,
	s3Key string,
	fpath string) error {

	if !fileutil.Exist(fpath) {
		return fmt.Errorf("file %q does not exist; failed to upload to %s/%s", fpath, bucket, s3Key)
	}
	stat, err := os.Stat(fpath)
	if err != nil {
		return err
	}
	size := humanize.Bytes(uint64(stat.Size()))

	lg.Info("uploading",
		zap.String("s3-bucket", bucket),
		zap.String("remote-path", s3Key),
		zap.String("file-size", size),
	)

	rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
	if err != nil {
		lg.Warn("failed to read a file", zap.String("file-path", fpath), zap.Error(err))
		return err
	}
	defer rf.Close()

	for i := 0; i < 5; i++ {
		_, err = s3API.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),

			Body: rf,

			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// vs. "public-read"
			ACL: aws.String("private"),

			Metadata: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
				"User": aws.String(user.Get()),
			},
		})
		if err == nil {
			lg.Info("uploaded",
				zap.String("s3-bucket", bucket),
				zap.String("remote-path", s3Key),
				zap.String("file-size", size),
			)
			break
		}

		lg.Warn("failed to upload",
			zap.String("s3-bucket", bucket),
			zap.String("remote-path", s3Key),
			zap.String("file-size", size),
			zap.Error(err),
			zap.Bool("error-expired-creds", request.IsErrorExpiredCreds(err)),
			zap.Bool("error-retriable", request.IsErrorRetryable(err)),
			zap.Bool("error-throttle", request.IsErrorThrottle(err)),
		)
		if request.IsErrorExpiredCreds(err) {
			break
		}
		if !request.IsErrorRetryable(err) && !request.IsErrorThrottle(err) {
			break
		}
		time.Sleep(time.Second * time.Duration(i+5))
	}

	return err
}

// UploadBody uploads the body reader to S3.
func UploadBody(
	lg *zap.Logger,
	s3API s3iface.S3API,
	bucket string,
	s3Key string,
	body io.ReadSeeker) (err error) {

	lg.Info("uploading",
		zap.String("s3-bucket", bucket),
		zap.String("remote-path", s3Key),
	)
	var output *s3.PutObjectOutput
	output, err = s3API.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),

		Body: body,

		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// vs. "public-read"
		ACL: aws.String("private"),

		Metadata: map[string]*string{
			"Kind": aws.String("aws-k8s-tester"),
			"User": aws.String(user.Get()),
		},
	})
	if err == nil {
		lg.Info("uploaded",
			zap.String("s3-bucket", bucket),
			zap.String("remote-path", s3Key),
			zap.String("version-id", aws.StringValue(output.VersionId)),
		)
	} else {
		lg.Warn("failed to upload",
			zap.String("s3-bucket", bucket),
			zap.String("remote-path", s3Key),
			zap.Error(err),
		)
	}
	return err
}

// EmptyBucket empties S3 bucket, by deleting all files in the bucket.
func EmptyBucket(lg *zap.Logger, s3API s3iface.S3API, bucket string) error {
	lg.Info("emptying bucket", zap.String("s3-bucket", bucket))
	batcher := s3manager.NewBatchDeleteWithClient(s3API)
	iter := &s3manager.DeleteListIterator{
		Bucket: aws.String(bucket),
		Paginator: request.Pagination{
			NewRequest: func() (*request.Request, error) {
				req, _ := s3API.ListObjectsRequest(&s3.ListObjectsInput{
					Bucket: aws.String(bucket),
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
				lg.Info("no such bucket", zap.String("s3-bucket", bucket), zap.Error(err))
				return nil
			}
		}
		lg.Warn("failed to empty bucket", zap.String("s3-bucket", bucket), zap.Error(err))
		return err
	}
	lg.Info("emptied bucket", zap.String("s3-bucket", bucket))
	return nil
}

// DeleteBucket deletes S3 bucket.
func DeleteBucket(lg *zap.Logger, s3API s3iface.S3API, bucket string) error {
	lg.Info("deleting bucket", zap.String("s3-bucket", bucket))
	_, err := s3API.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				lg.Info("no such bucket", zap.String("s3-bucket", bucket), zap.Error(err))
				return nil
			}
		}
		lg.Warn("failed to delete bucket", zap.String("s3-bucket", bucket), zap.Error(err))
	}

	lg.Info("deleted bucket", zap.String("s3-bucket", bucket))
	return nil
}

// ListInDescendingLastModified returns s3 objects which are sorted
// in "descending" order of last modified timestamps.
// That is, the first element in the response is of the "most" recent
// and highest last modified timestamp value.
func ListInDescendingLastModified(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3KeyPfx string, opts ...OpOption) (s3Objects []*s3.Object, err error) {
	ret := Op{verbose: false, overwrite: false}
	ret.applyOpts(opts)

	lg.Info("listing objects", zap.String("s3-bucket", bucket), zap.String("s3-key-prefix", s3KeyPfx))
	s3Objects = make([]*s3.Object, 0)
	err = s3API.ListObjectsV2Pages(
		&s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(s3KeyPfx),
		},
		func(resp *s3.ListObjectsV2Output, lastPage bool) bool {
			s3Objects = append(s3Objects, resp.Contents...)
			return true
		},
	)
	if err != nil {
		lg.Warn("failed to list objects", zap.String("s3-bucket", bucket), zap.String("s3-key-prefix", s3KeyPfx), zap.Error(err))
		return nil, err
	}

	// sort in "LastModified" descending order
	sort.Slice(s3Objects, func(i, j int) bool {
		t1 := aws.TimeValue(s3Objects[i].LastModified)
		t2 := aws.TimeValue(s3Objects[j].LastModified)
		// sort.Interface.Less; index i should sort before the element with index j
		return t1.After(t2)
	})
	lg.Info("listed objects",
		zap.String("s3-bucket", bucket),
		zap.String("s3-key-prefix", s3KeyPfx),
		zap.Int("s3-objects", len(s3Objects)),
	)
	return s3Objects, nil
}

// Exist returns true if the object exists.
func Exist(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3Key string, opts ...OpOption) (exist bool, err error) {
	ret := Op{verbose: false, overwrite: false}
	ret.applyOpts(opts)

	lg.Info("checking object", zap.String("s3-bucket", bucket), zap.String("s3-key", s3Key))
	resp, err := s3API.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		lg.Warn("failed to head object", zap.String("s3-bucket", bucket), zap.String("s3-key", s3Key), zap.Error(err))
		return false, err
	}
	size := humanize.Bytes(uint64(aws.Int64Value(resp.ContentLength)))
	lg.Info("checked object",
		zap.String("s3-bucket", bucket),
		zap.String("s3-key", s3Key),
		zap.String("size", size),
	)
	return true, nil
}

// HeadObjectStatus represents the S3 object head status.
type HeadObjectStatus struct {
	HeadObject *s3.HeadObjectOutput
	Error      error
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	return awsErr.Code() == "NotFound"
}

// PollUntilExist waits until the object exists.
func PollUntilExist(
	ctx context.Context,
	stopc chan struct{},
	lg *zap.Logger,
	s3API s3iface.S3API,
	bucket string,
	s3Key string,
	initialWait time.Duration,
	pollInterval time.Duration,
) <-chan HeadObjectStatus {
	now := time.Now()

	lg.Info("polling object",
		zap.String("s3-bucket", bucket),
		zap.String("s3-key", s3Key),
	)
	ch := make(chan HeadObjectStatus, 10)
	go func() {
		// very first poll should be no-wait
		// in case stack has already reached desired status
		// wait from second interation
		interval := time.Duration(0)

		first := true
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
				ch <- HeadObjectStatus{HeadObject: nil, Error: ctx.Err()}
				close(ch)
				return

			case <-stopc:
				lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
				ch <- HeadObjectStatus{HeadObject: nil, Error: errors.New("wait stopped")}
				close(ch)
				return

			case <-time.After(interval):
				// very first poll should be no-wait
				// in case stack has already reached desired status
				// wait from second interation
				if interval == time.Duration(0) {
					interval = pollInterval
				}
			}

			obj, err := s3API.HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(s3Key),
			})
			if err == nil {
				lg.Info("found object", zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")))
				ch <- HeadObjectStatus{HeadObject: obj, Error: nil}
				close(ch)
				return
			}

			lg.Warn("object not found; retrying",
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
				zap.Error(err),
			)
			if isNotFound(err) {
				err = nil
			}
			ch <- HeadObjectStatus{HeadObject: nil, Error: err}

			if first {
				lg.Info("sleeping", zap.Duration("initial-wait", initialWait))

				select {
				case <-ctx.Done():
					lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
					ch <- HeadObjectStatus{HeadObject: nil, Error: ctx.Err()}
					close(ch)
					return

				case <-stopc:
					lg.Warn("wait stopped, stopc closed", zap.Error(ctx.Err()))
					ch <- HeadObjectStatus{HeadObject: nil, Error: errors.New("wait stopped")}
					close(ch)
					return

				case <-time.After(initialWait):
				}
				first = false
			}
		}

		lg.Warn("wait aborted, ctx done", zap.Error(ctx.Err()))
		ch <- HeadObjectStatus{HeadObject: nil, Error: ctx.Err()}
		close(ch)
		return
	}()
	return ch
}

// Download downloads the file from the S3 bucket.
func Download(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3Key string, localPath string, opts ...OpOption) (err error) {
	return download(lg, s3API, bucket, s3Key, localPath, opts...)
}

// DownloadToTempFile downloads the file from the S3 bucket to a temporary file.
func DownloadToTempFile(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3Key string, opts ...OpOption) (localPath string, err error) {
	localPath = fileutil.GetTempFilePath()
	return localPath, download(lg, s3API, bucket, s3Key, localPath, opts...)
}

func download(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3Key string, localPath string, opts ...OpOption) (err error) {
	ret := Op{verbose: false, overwrite: false}
	ret.applyOpts(opts)

	lg.Info("downloading object",
		zap.String("s3-bucket", bucket),
		zap.String("s3-key", s3Key),
		zap.String("timeout", ret.timeout.String()),
	)
	ctx, reqOpts := context.Background(), make([]request.Option, 0)
	if ret.timeout > 0 {
		var cancelFunc func()
		ctx, cancelFunc = context.WithTimeout(context.Background(), ret.timeout)
		defer cancelFunc()
		reqOpts = append(reqOpts, request.WithResponseReadTimeout(ret.timeout))
	}
	resp, err := s3API.GetObjectWithContext(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
		},
		reqOpts...,
	)
	if err != nil {
		lg.Warn("failed to get object", zap.String("s3-bucket", bucket), zap.String("s3-key", s3Key), zap.Error(err))
		return err
	}

	if err = os.MkdirAll(filepath.Dir(localPath), 0700); err != nil {
		lg.Warn("failed to mkdir", zap.String("s3-key", s3Key), zap.Error(err))
		return err
	}
	if fileutil.Exist(localPath) {
		lg.Warn("local file path already exists", zap.String("local-path", localPath))
		if !ret.overwrite {
			return fmt.Errorf("local file %q already exists; can't overwrite", localPath)
		}
	}
	f, err := os.OpenFile(localPath, os.O_RDWR|os.O_TRUNC, 0777)
	if err != nil {
		f, err = os.Create(localPath)
		if err != nil {
			lg.Warn("failed to write file", zap.String("s3-bucket", bucket), zap.String("s3-key", s3Key), zap.Error(err))
			return err
		}
	}
	n, err := io.Copy(f, resp.Body)
	f.Close()
	resp.Body.Close()
	if err != nil {
		lg.Warn("failed to download object",
			zap.String("s3-bucket", bucket),
			zap.String("s3-key", s3Key),
			zap.Error(err),
		)
		return err
	}

	lg.Info("downloaded object",
		zap.String("s3-bucket", bucket),
		zap.String("s3-key", s3Key),
		zap.String("object-size", humanize.Bytes(uint64(n))),
		zap.String("local-path", localPath),
	)
	return nil
}

// DownloadDir downloads all files from the directory in the S3 bucket.
func DownloadDir(lg *zap.Logger, s3API s3iface.S3API, bucket string, s3Dir string, opts ...OpOption) (targetDir string, err error) {
	ret := Op{verbose: false, overwrite: false}
	ret.applyOpts(opts)

	s3Dir = path.Clean(s3Dir) + "/"
	dirPfx := "download-s3-bucket-dir-" + bucket + s3Dir
	dirPfx = strings.Replace(dirPfx, "/", "", -1)
	lg.Info("creating temp dir", zap.String("dir-prefix", dirPfx))
	targetDir = fileutil.MkTmpDir(os.TempDir(), dirPfx)

	lg.Info("downloading directory from bucket",
		zap.String("s3-bucket", bucket),
		zap.String("s3-dir", s3Dir),
		zap.String("target-dir", targetDir),
	)
	objects := make([]*s3.Object, 0, 100)
	pageNum := 0
	err = s3API.ListObjectsPages(
		&s3.ListObjectsInput{
			Bucket: aws.String(bucket),
			Prefix: aws.String(s3Dir),
		},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			objects = append(objects, page.Contents...)
			pageNum++
			lg.Info("listing",
				zap.String("s3-bucket", bucket),
				zap.Int("page-num", pageNum),
				zap.Bool("last-page", lastPage),
				zap.Int("returned-objects", len(page.Contents)),
				zap.Int("total-objects", len(objects)),
			)
			return true
		},
	)
	if err != nil {
		os.RemoveAll(targetDir)
		return "", err
	}
	for _, obj := range objects {
		time.Sleep(300 * time.Millisecond)

		s3Key := aws.StringValue(obj.Key)
		lg.Info("downloading object",
			zap.String("s3-key", s3Key),
			zap.String("object-size", humanize.Bytes(uint64(aws.Int64Value(obj.Size)))),
		)
		resp, err := s3API.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    obj.Key,
		})
		if err != nil {
			lg.Warn("failed to get object", zap.String("s3-key", s3Key), zap.Error(err))
			continue
		}
		fpath := filepath.Join(targetDir, s3Key)
		if err = os.MkdirAll(filepath.Dir(fpath), 0700); err != nil {
			lg.Warn("failed to mkdir", zap.String("s3-key", s3Key), zap.Error(err))
			continue
		}
		f, err := os.OpenFile(fpath, os.O_RDWR|os.O_TRUNC, 0777)
		if err != nil {
			f, err = os.Create(fpath)
			if err != nil {
				lg.Warn("failed to write file", zap.String("s3-key", s3Key), zap.Error(err))
				continue
			}
		}
		n, err := io.Copy(f, resp.Body)
		f.Close()
		resp.Body.Close()
		if err == nil {
			lg.Info("downloaded object",
				zap.String("s3-key", s3Key),
				zap.String("object-size", humanize.Bytes(uint64(aws.Int64Value(obj.Size)))),
				zap.String("copied-size", humanize.Bytes(uint64(n))),
			)
		} else {
			lg.Warn("failed to download object",
				zap.String("s3-key", s3Key),
				zap.String("object-size", humanize.Bytes(uint64(aws.Int64Value(obj.Size)))),
				zap.String("copied-size", humanize.Bytes(uint64(n))),
				zap.Error(err),
			)
		}
	}
	lg.Info("downloaded directory from bucket",
		zap.String("s3-bucket", bucket),
		zap.String("s3-dir", s3Dir),
		zap.String("target-dir", targetDir),
	)
	return targetDir, nil
}

// Op represents a SSH operation.
type Op struct {
	verbose   bool
	overwrite bool
	timeout   time.Duration
}

// OpOption configures archiver operations.
type OpOption func(*Op)

// WithVerbose configures verbose level in SSH operations.
func WithVerbose(b bool) OpOption {
	return func(op *Op) { op.verbose = b }
}

// WithOverwrite configures overwrites.
func WithOverwrite(b bool) OpOption {
	return func(op *Op) { op.overwrite = b }
}

// WithTimeout configures request timeouts.
func WithTimeout(timeout time.Duration) OpOption {
	return func(op *Op) { op.timeout = timeout }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
