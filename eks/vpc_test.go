package eks

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestEmbeddedVPCStack(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	f, err := ioutil.TempFile(os.TempDir(), "a8-eksconfig")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath, _ = filepath.Abs(f.Name())
	f.Close()
	os.RemoveAll(cfg.ConfigPath)
	cfg.ValidateAndSetDefaults()

	ek, err := newTesterEmbedded(cfg)
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
