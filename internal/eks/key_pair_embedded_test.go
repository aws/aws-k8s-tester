package eks

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/awstester/eksconfig"
)

func TestEmbeddedCreateKeyPair(t *testing.T) {
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
		t.Fatalf("expected '*embedded', got %v", reflect.TypeOf(ek))
	}

	if err = md.createKeyPair(); err != nil {
		t.Fatal(err)
	}
	if err = md.deleteKeyPair(); err != nil {
		t.Fatal(err)
	}
}
