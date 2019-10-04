package kms

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-k8s-tester/kmsconfig"
)

func TestDeployer(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		t.Skip()
	}

	cfg := kmsconfig.NewDefault()
	dp, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = dp.CreateKey(); err != nil {
		t.Fatal(err)
	}

	keys, err := dp.ListAllKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		fmt.Printf("%+v\n", k)
	}

	if !cfg.KeyMetadata.Enabled {
		t.Fatalf("KeyMetadata.Enabled unexpected %v", cfg.KeyMetadata.Enabled)
	}

	if err = dp.ScheduleKeyDeletion(7); err != nil {
		t.Fatal(err)
	}
	if cfg.KeyMetadata.KeyState != "PendingDeletion" {
		t.Fatalf("KeyMetadata.KeyState unexpected %q", cfg.KeyMetadata.KeyState)
	}
}
