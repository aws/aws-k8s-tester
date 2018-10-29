package eks

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/awstester/eksconfig"
)

func TestEmbeddedServiceRole(t *testing.T) {
	if os.Getenv("RUN_AWS_UNIT_TESTS") != "1" {
		t.Skip()
	}

	cfg := eksconfig.NewDefault()

	ek, err := newTesterEmbedded(cfg)
	if err != nil {
		t.Fatal(err)
	}
	md, ok := ek.(*embedded)
	if !ok {
		t.Fatalf("expected *embedded, got %v", reflect.TypeOf(ek))
	}

	defer func() {
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
