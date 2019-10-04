package kms

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-k8s-tester/kmsconfig"
)

func TestDeployer(t *testing.T) {
	if os.Getenv("RUN_AWS_TESTS") != "1" {
		// t.Skip()
	}

	cfg := kmsconfig.NewDefault()
	dp, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err = dp.CreateKey(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err = dp.ScheduleKeyDeletion(7); err != nil {
			t.Fatal(err)
		}
	}()

	keys, err := dp.ListAllKeys()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		fmt.Printf("%+v\n", k)
	}

	if err = dp.DisableKey(); err != nil {
		t.Fatal(err)
	}
	if err = dp.EnableKey(); err != nil {
		t.Fatal(err)
	}

	if err = dp.EnableKeyRotation(); err != nil {
		t.Fatal(err)
	}
	if err = dp.DisableKeyRotation(); err != nil {
		t.Fatal(err)
	}

	// The key argument should be the AES key,
	// either 16 or 32 bytes to select AES-128 or AES-256
	dataKeyCtx := map[string]string{
		"Kind":        "aws-k8s-tester",
		"Description": cfg.ID,
	}
	cipherKey1, plainKey1, err := dp.GenerateDataKey(dataKeyCtx, "AES_256", 0)
	if err != nil {
		t.Fatal(err)
	}
	var plainKey2 []byte
	plainKey2, err = dp.Decrypt(dataKeyCtx, cipherKey1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plainKey1) != 32 {
		t.Fatalf("len(plainKey1) expected 32, got %d", len(plainKey1))
	}
	if !bytes.Equal(plainKey1, plainKey2) {
		t.Fatalf("expected plain key %q, got %q", string(plainKey1), string(plainKey2))
	}

	encryptionCtx := map[string]string{"a": "b"}
	plainTxt1 := []byte("Hello World! This is great!")
	var cipherTxt []byte
	cipherTxt, err = dp.Encrypt(encryptionCtx, plainTxt1)
	if err != nil {
		t.Fatal(err)
	}
	var plainTxt2 []byte
	plainTxt2, err = dp.Decrypt(encryptionCtx, cipherTxt)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plainTxt1, plainTxt2) {
		t.Fatalf("decrypted text expected %q, got %q", string(plainTxt1), string(plainTxt2))
	}
}
