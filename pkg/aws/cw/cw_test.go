package cw

import (
	"testing"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"go.uber.org/zap"
)

func TestCW(t *testing.T) {
	t.Skip()

	lg := zap.NewExample()
	ss, _, _, err := pkg_aws.New(&pkg_aws.Config{
		Logger:    lg,
		Partition: "aws",
		Region:    "us-west-2",
	})
	if err != nil {
		t.Skip(err)
	}
	fpath := fileutil.GetTempFilePath()
	fpath += ".png"

	cwAPI := cloudwatch.New(ss)
	if err = GetMetricsImage(lg, cwAPI, query, fpath); err != nil {
		t.Fatal(err)
	}
}

const query = `
`
