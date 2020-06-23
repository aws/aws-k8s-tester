package cw

import (
	"testing"
	"time"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
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

	cwAPI := cloudwatch.New(ss)

	if err = PutData(
		lg,
		cwAPI,
		"leegyuho",
		10,
		&cloudwatch.MetricDatum{
			Timestamp:  aws.Time(time.Now()),
			MetricName: aws.String("test"),
			Value:      aws.Float64(123),
			Unit:       aws.String(cloudwatch.StandardUnitMilliseconds),
		},
	); err != nil {
		t.Fatal(err)
	}

	t.Skip()

	fpath := fileutil.GetTempFilePath()
	fpath += ".png"
	if err = GetMetricsImage(lg, cwAPI, query, fpath); err != nil {
		t.Fatal(err)
	}
}

const query = `
`
