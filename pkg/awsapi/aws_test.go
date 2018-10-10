package awsapi

import (
	"testing"

	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	t.Skip()

	if _, err := New(nil); err == nil {
		t.Fatal("expected error, got nil")
	}
	ss, err := New(&Config{
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
}
