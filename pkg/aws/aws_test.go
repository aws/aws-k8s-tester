package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	// t.Skip()

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
	so, err := sm.GetParameters(&ssm.GetParametersInput{Names: aws.StringSlice([]string{"/aws/service/eks/optimized-ami/1.14/amazon-linux-2/recommended"})})
	if err != nil {
		panic(err)
	}
	fmt.Println(so)
}
