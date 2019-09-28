package eks

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestEmbeddedCreateKeyPair(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	f, err := ioutil.TempFile(os.TempDir(), "aws-k8s-tester-eksconfig")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath, _ = filepath.Abs(f.Name())
	f.Close()
	os.RemoveAll(cfg.ConfigPath)
	cfg.ValidateAndSetDefaults()

	ek, err := NewTester(cfg)
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
