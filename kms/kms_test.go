package kms

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"unsafe"

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

	// https://github.com/kubernetes/kubernetes/blob/13b46eb9d4783d4070a0ff50cfb290f6b0a875f7/staging/src/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/envelope.go#L140-L151
	newKey, err := generateKey(32)
	if err != nil {
		t.Fatal(err)
	}
	encKey, err := dp.Encrypt(nil, newKey)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("encKey size:", len(encKey)) // 184

	println()
	encodedDEK := base64.StdEncoding.EncodeToString(encKey)
	fmt.Println("cache:", encodedDEK)
	fmt.Println("encodedDEK size:", len(encodedDEK))             // 248
	fmt.Println("generateKey size:", unsafe.Sizeof(generateKey)) // 8

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

// generateKey generates a random key using system randomness.
func generateKey(length int) (key []byte, err error) {
	key = make([]byte, length)
	if _, err = rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
