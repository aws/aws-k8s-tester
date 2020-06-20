package s3

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
)

func TestS3(t *testing.T) {
	lg := zap.NewExample()
	ss, _, _, err := pkg_aws.New(&pkg_aws.Config{
		Logger:    lg,
		Partition: "aws",
		Region:    "us-west-2",
	})
	if err != nil {
		t.Skip(err)
	}
	s3API := s3.New(ss)

	bucket := randutil.String(15)
	dir := filepath.Join("hello", "world")
	if err = CreateBucket(lg, s3API, bucket, "us-west-2", "", 0); err != nil {
		t.Fatal(err)
	}
	defer func() {
		t.Logf("EmptyBucket: %v", EmptyBucket(lg, s3API, bucket))
		t.Logf("DeleteBucket: %v", DeleteBucket(lg, s3API, bucket))
	}()

	for i := 0; i < 10; i++ {
		if err = UploadBody(
			lg,
			s3API,
			bucket,
			filepath.Join(dir, randutil.String(10)),
			bytes.NewReader(randutil.Bytes(10)),
		); err != nil {
			t.Fatal(err)
		}
	}

	targetDir, err := DownloadDir(lg, s3API, bucket, dir)
	fmt.Println("targetDir:", targetDir)
	defer os.RemoveAll(targetDir)
	if err != nil {
		t.Fatal(err)
	}
}
