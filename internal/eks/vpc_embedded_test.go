package eks

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/awstester/eksconfig"
)

func TestEmbeddedVPCStack(t *testing.T) {
	if os.Getenv("RUN_AWS_UNIT_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	ek, err := NewAWSTester(cfg)
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
	if err = md.deleteVPC(); err != nil {
		t.Fatal(err)
	}
}
