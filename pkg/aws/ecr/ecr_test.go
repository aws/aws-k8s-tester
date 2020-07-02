package ecr

import (
	"fmt"
	"testing"

	pkg_aws "github.com/aws/aws-k8s-tester/pkg/aws"
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
	img, err := Check(lg, ecrAPI, "607362164682", "us-east-1", "aws/aws-k8s-tester", "latest")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(img)
}
