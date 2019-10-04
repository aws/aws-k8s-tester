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

	ctx := map[string]string{
		"Kind":        "aws-k8s-tester",
		"Description": cfg.ID,
	}

	cipherKey, plainKey, err := dp.GenerateDataKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("cipherKey:", string(cipherKey))
	fmt.Println("plainKey:", string(plainKey))

	// TODO: test

	encryptionCtx := map[string]string{
		"a": "b",
	}
	plain := []byte("Hello World! This is great!")

	var cipher []byte
	cipher, err = dp.Encrypt(encryptionCtx, plain)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("cipher:", string(cipher))

	var decrypted []byte
	decrypted, err = dp.Decrypt(encryptionCtx, cipher)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(plain, decrypted) {
		t.Fatalf("decrypted text expected %q, got %q", string(plain), string(decrypted))
	}
}
