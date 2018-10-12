// Package wrk implements wrk related utilities.
package wrk

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/awstester/pkg/awsapi"
	"github.com/aws/awstester/pkg/wrk"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/sts"
	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func init() {
	cobra.EnablePrefixMatching = true
}

// NewCommand implements "awstest wrk" command.
func NewCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "wrk",
		Short: "wrk commands",
		Run:   runFunc,
	}
	rootCmd.AddCommand(
		newAverage(),
		newMerge(),
		newConvert(),
	)
	rootCmd.PersistentFlags().StringVar(&output, "output", "", "output file path")

	rootCmd.PersistentFlags().BoolVar(&outputCSV, "output-csv", true, "'true' to output results in CSV")
	rootCmd.PersistentFlags().BoolVar(&outputS3Upload, "output-s3-upload", false, "'true' to upload wrk outputs")
	rootCmd.PersistentFlags().StringVar(&outputS3UploadDir, "output-s3-upload-directory", "test", "directory to upload output file")
	rootCmd.PersistentFlags().StringVar(&outputS3UploadRegion, "output-s3-upload-region", "us-west-2", "AWS region for S3 uploads")

	rootCmd.PersistentFlags().IntVar(&wrkCfg.StartAtMinute, "start-at-minute", 0, "minute to start the command (temporary dumb feature to be removed after batch integration...)")

	rootCmd.PersistentFlags().StringVar(&wrkCfg.Endpoint, "endpoint", "", "wrk command endpoint")
	rootCmd.PersistentFlags().IntVar(&wrkCfg.Threads, "threads", 2, "number of threads")
	rootCmd.PersistentFlags().IntVar(&wrkCfg.Connections, "connections", 200, "number of connections")
	rootCmd.PersistentFlags().DurationVar(&wrkCfg.Duration, "duration", 15*time.Second, "duration to run 'wrk' command")
	return rootCmd
}

var (
	output               string
	outputCSV            bool
	outputS3Upload       bool
	outputS3UploadDir    string
	outputS3UploadRegion string
	wrkCfg               wrk.Config
)

func runFunc(cmd *cobra.Command, args []string) {
	if output == "" {
		fmt.Fprintln(os.Stderr, "output path is not specified")
		os.Exit(1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger (%v)\n", err)
		os.Exit(1)
	}
	wrkCfg.Logger = lg

	rs, err := wrk.Run(wrkCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run wrk (%v)\n", err)
		os.Exit(1)
	}

	if outputCSV {
		if err = wrk.ToCSV(output, rs); err != nil {
			fmt.Fprintf(os.Stderr, "failed to convert to CSV %q (%v)\n", output, err)
			os.Exit(1)
		}
	} else {
		var f *os.File
		f, err = os.OpenFile(output, os.O_RDWR|os.O_TRUNC, 0600)
		if err != nil {
			f, err = os.Create(output)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create file %q (%v)\n", output, err)
				os.Exit(1)
			}
		}
		defer f.Close()
		if _, err = f.Write([]byte(rs.Output)); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write to file %q (%v)\n", output, err)
			os.Exit(1)
		}
	}

	awsCfg := &awsapi.Config{
		Logger:        lg,
		DebugAPICalls: false,
		Region:        outputS3UploadRegion,
	}
	var ss *session.Session
	ss, err = awsapi.New(awsCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create AWS API (%v)\n", err)
		os.Exit(1)
	}
	st := sts.New(ss)
	var so *sts.GetCallerIdentityOutput
	so, err = st.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get caller identity (%v)\n", err)
		os.Exit(1)
	}
	up := &uploader{
		bucket: getBucket(*so.Account),
		lg:     lg,
		s3:     s3.New(ss),
	}

	s3Path := getName()
	if outputCSV {
		s3Path += ".csv"
	}
	if err = up.upload(output, filepath.Join(outputS3UploadDir, s3Path)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to upload %q (%v)\n", output, err)
		os.Exit(1)
	}
}

type uploader struct {
	bucket string
	lg     *zap.Logger
	s3     s3iface.S3API
}

func (up *uploader) upload(localPath, s3Path string) error {
	bucket := up.bucket
	_, err := up.s3.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(outputS3UploadRegion),
		},
		// TODO: enable this when open-sourced, to make all logs available to communities
		// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
		// ACL: aws.String("public-read"),
		ACL: aws.String("private"),
	})
	if err != nil {
		exist := false
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists:
				up.lg.Debug("bucket already exists", zap.String("bucket", bucket), zap.Error(err))
				exist = true
			case s3.ErrCodeBucketAlreadyOwnedByYou:
				up.lg.Debug("bucket already owned by me", zap.String("bucket", bucket), zap.Error(err))
				exist = true
			default:
				return err
			}
		}
		if !exist {
			return err
		}
	}
	up.lg.Info("created bucket", zap.String("bucket", bucket))

	h, _ := os.Hostname()
	tags := []*s3.Tag{{Key: aws.String("HOSTNAME"), Value: aws.String(h)}}
	_, err = up.s3.PutBucketTagging(&s3.PutBucketTaggingInput{
		Bucket:  aws.String(bucket),
		Tagging: &s3.Tagging{TagSet: tags},
	})
	if err != nil {
		return err
	}

	d, err := ioutil.ReadFile(localPath)
	if err != nil {
		return err
	}

	hn, _ := os.Hostname()
	for i := 0; i < 30; i++ {
		_, err = up.s3.PutObject(&s3.PutObjectInput{
			Bucket:  aws.String(bucket),
			Key:     aws.String(s3Path),
			Body:    bytes.NewReader(d),
			Expires: aws.Time(time.Now().UTC().Add(24 * time.Hour)),

			// TODO: enable this when open-sourced, to make all logs available to communities
			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// ACL: aws.String("public-read"),
			ACL: aws.String("private"),

			Metadata: map[string]*string{
				bucket:     aws.String(bucket),
				"HOSTNAME": aws.String(hn),
			},
		})
		if err == nil {
			up.lg.Info("uploaded",
				zap.String("bucket", bucket),
				zap.String("local-path", localPath),
				zap.String("remote-path", s3Path),
				zap.String("size", humanize.Bytes(uint64(len(d)))),
			)
			break
		}

		if aerr, ok := err.(awserr.Error); ok {
			up.lg.Warn("failed to upload",
				zap.String("bucket", bucket),
				zap.String("local-path", localPath),
				zap.String("remote-path", s3Path),
				zap.String("size", humanize.Bytes(uint64(len(d)))),
				zap.String("error-code", aerr.Code()),
				zap.Error(err),
			)
		} else {
			up.lg.Warn("failed to upload",
				zap.String("bucket", bucket),
				zap.String("local-path", localPath),
				zap.String("remote-path", s3Path),
				zap.String("size", humanize.Bytes(uint64(len(d)))),
				zap.Error(err),
			)
		}

		time.Sleep(15 * time.Second)
	}
	return err
}

func getBucket(accountID string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s-awstester-wrk-%d%02d%02d", accountID, now.Year(), now.Month(), now.Day())
}

func getName() (name string) {
	resp, err := http.Get("http://169.254.169.254/latest/meta-data/public-ipv4")
	if err != nil {
		resp, err = http.Get("http://169.254.169.254/latest/meta-data/local-ipv4")
		if err != nil {
			name = randString(15)
			return name
		}
	}

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		name = randString(15)
		return name
	}

	if len(d) == 0 {
		resp, err = http.Get("http://169.254.169.254/latest/meta-data/local-ipv4")
		if err != nil {
			name = randString(15)
			return name
		}

		d, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			name = randString(15)
			return name
		}
	}

	resp.Body.Close()

	name = strings.TrimSpace(reg.ReplaceAllString(string(d), ""))
	return name
}

var reg *regexp.Regexp

func init() {
	var err error
	reg, err = regexp.Compile("[^0-9]+")
	if err != nil {
		panic(err)
	}
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UTC().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
