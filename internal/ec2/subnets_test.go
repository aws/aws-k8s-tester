package ec2

import (
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-k8s-tester/ec2config"
)

func TestSubnets(t *testing.T) {
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

	if err = md.createSubnets(); err != nil {
		t.Fatal(err)
	}
	subnets1 := md.cfg.SubnetIDs
	md.cfg.SubnetIDs = nil

	if err = md.getSubnets(); err != nil {
		t.Fatal(err)
	}
	subnets2 := md.cfg.SubnetIDs

	if !reflect.DeepEqual(subnets1, subnets2) {
		t.Fatalf("subnets are different %v != %v", subnets1, subnets2)
	}

	if err = md.deleteSubnet(); err != nil {
		t.Fatal(err)
	}
	if err = md.deleteVPC(); err != nil {
		t.Fatal(err)
	}
}
