package pki

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRSA(t *testing.T) {
	k, err := NewRSA(2048)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := k.PrivateKeyBytes()
	publicKey := k.PublicKeyBytes()
	fmt.Println(string(privateKey))
	fmt.Println(string(publicKey))

	msg := []byte("Hello World!")

	sig, err := k.SignPayloadWithPrivateKey(msg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(sig))
	if err = k.VerifyPayloadWithPublicKey(msg, sig); err != nil {
		t.Fatal(err)
	}

	cipherText, err := k.EncryptWithPublicKey(msg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(cipherText))

	decrypted1, err := k.DecryptWithPrivateKey(cipherText)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, decrypted1) {
		t.Fatalf("expected %q, got %q", string(msg), string(decrypted1))
	}

	k2, err := LoadRSA(k.PrivateKeyBytes())
	if err != nil {
		t.Fatal(err)
	}
	decrypted2, err := k2.DecryptWithPrivateKey(cipherText)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(msg, decrypted2) {
		t.Fatalf("expected %q, got %q", string(msg), string(decrypted2))
	}

	if err = k.SignRootCertificate(); err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(k.RootCertificateBytes()))
}
