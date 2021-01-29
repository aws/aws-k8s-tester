package aws

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/service/ssm"
	"go.uber.org/zap"
)

func TestAMI(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	if _, _, _, err := New(nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	ss, _, _, err := New(&Config{
		Logger: zap.NewExample(),
		Region: "us-west-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ss.Config == nil {
		t.Skip("cannot create session; nil Config")
	}
	if ss.Config.Credentials == nil {
		t.Skip("cannot create session; nil Credentials")
	}
	creds, err := ss.Config.Credentials.Get()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("access key: %d bytes", len(creds.AccessKeyID))
	t.Logf("secret key: %d bytes", len(creds.SecretAccessKey))

	sm := ssm.New(ss)

	a1, err := FetchAMI(sm, "/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", a1)

	a2, err := FetchAMI(sm, "/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", a2)

	a3, err := FetchAMI(sm, "/aws/service/eks/optimized-ami/1.15/amazon-linux-2-gpu/recommended/image_id")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", a3)

	a4, err := FetchAMI(sm, "/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", a4)
}
