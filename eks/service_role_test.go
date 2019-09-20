package eks

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func TestEmbeddedServiceRole(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}

	ek, err := newTesterEmbedded(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := ek.(*embedded)
	if !ok {
		t.Fatalf("expected *embedded, got %v", reflect.TypeOf(ek))
	}

	defer func() {
		fmt.Println("waiting...")
		time.Sleep(3 * time.Minute)

		if err = md.detachPolicyForAWSServiceRoleForAmazonEKS(); err != nil {
			t.Log(err)
		}
		if err = md.deleteAWSServiceRoleForAmazonEKS(); err != nil {
			t.Log(err)
		}
	}()

	if err = md.createAWSServiceRoleForAmazonEKS(); err != nil {
		t.Fatal(err)
	}
	if err = md.attachPolicyForAWSServiceRoleForAmazonEKS(); err != nil {
		t.Fatal(err)
	}
}
