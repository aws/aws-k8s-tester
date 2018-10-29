package ec2

import (
	"os"
	"reflect"
	"testing"

	ec2config "github.com/aws/awstester/internal/ec2/config"
)

func TestVPC(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := ec2config.NewDefault()

	ec, err := NewDeployer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := ec.(*embedded)
	if !ok {
		t.Fatalf("expected '*embedded', got %v", reflect.TypeOf(ec))
	}

	if err = md.createVPC(); err != nil {
		t.Fatal(err)
	}
	if err = md.deleteVPC(); err != nil {
		t.Fatal(err)
	}
}
