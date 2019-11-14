package eks

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestVPCStack(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	ek, err := NewTester(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := ek.(*embedded)
	if !ok {
		t.Fatalf("expected *embedded, got %v", reflect.TypeOf(ek))
	}

	if err = md.createVPC(); err != nil {
		t.Fatal(err)
	}

	fmt.Println("waiting...")
	time.Sleep(3 * time.Minute)

	if err = md.deleteVPC(); err != nil {
		t.Fatal(err)
	}
}
