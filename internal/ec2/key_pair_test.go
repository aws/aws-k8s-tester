package ec2

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-k8s-tester/ec2config"
)

func TestKey(t *testing.T) {
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

	if err = md.createKeyPair(); err != nil {
		t.Fatal(err)
	}
	if err = md.deleteKeyPair(); err != nil {
		t.Fatal(err)
	}
}
