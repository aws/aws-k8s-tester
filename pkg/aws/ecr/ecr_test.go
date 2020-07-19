package ecr

import (
	"fmt"
	"testing"
	"time"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
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

	ecrAPI := ecr.New(ss, aws.NewConfig().WithRegion("us-east-1"))
	img, ok, err := Check(lg, ecrAPI, "607362164682", "us-east-1", "aws/aws-k8s-tester", "latest")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(img, ok)
}

func TestCWCreateDelete(t *testing.T) {
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

	repoName := randutil.String(10)
	ecrAPI := ecr.New(ss, aws.NewConfig().WithRegion("us-west-2"))

	repoURI, err := Create(
		lg,
		ecrAPI,
		"607362164682",
		"us-west-2",
		repoName,
		false,
		ecr.ImageTagMutabilityMutable,
		testRepoPolicy,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(repoURI)

	repoURI, err = Create(
		lg,
		ecrAPI,
		"607362164682",
		"us-west-2",
		repoName,
		false,
		ecr.ImageTagMutabilityMutable,
		testRepoPolicy,
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(repoURI)

	time.Sleep(10 * time.Second)
	err = Delete(lg, ecrAPI, "607362164682", "us-west-2", repoName, true)
	if err != nil {
		t.Fatal(err)
	}
}

const testRepoPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
	  {
		"Sid": "AllowPushPull",
		"Effect": "Allow",
		"Principal": {
		  "Service": [
			"eks.amazonaws.com"
		  ]
		},
		"Action": [
		  "ecr:BatchCheckLayerAvailability",
		  "ecr:BatchGetImage",
		  "ecr:DescribeImages",
		  "ecr:DescribeRepositories",
		  "ecr:GetDownloadUrlForLayer"
		]
	  }
	]
}
`
