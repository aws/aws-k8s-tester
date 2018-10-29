package s3

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/pkg/awsapi"

	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
)

/*
RUN_AWS_TESTS=1 go test -v -run TestEmbedded
*/
func TestEmbedded(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	awsCfg := &awsapi.Config{
		Logger:         zap.NewExample(),
		DebugAPICalls:  cfg.LogDebug,
		Region:         cfg.AWSRegion,
		CustomEndpoint: cfg.AWSCustomEndpoint,
	}
	ss, err := awsapi.New(awsCfg)
	if err != nil {
		t.Fatal(err)
	}

	sp := NewEmbedded(zap.NewExample(), cfg, s3.New(ss))
	if err = sp.CreateBucketForAccessLogs(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = sp.DeleteBucket(sp.BucketForAccessLogs()); err != nil {
			t.Log(err)
		}
		if err = sp.DeleteBucket(sp.BucketForTests()); err != nil {
			t.Log(err)
		}
	}()

	f, ferr := ioutil.TempFile(os.TempDir(), "test")
	if ferr != nil {
		t.Fatal(ferr)
	}
	_, err = f.Write([]byte("hello world!"))
	if err != nil {
		t.Fatal(err)
	}
	localPath := f.Name()
	f.Close()
	defer os.RemoveAll(localPath)

	if err = sp.UploadToBucketForTests(localPath, "hello-world"); err != nil {
		t.Fatal(err)
	}
}
