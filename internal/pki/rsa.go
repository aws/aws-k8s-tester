package pki

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type RSA struct {
	privateKey *rsa.PrivateKey
	rootCert   *x509.Certificate
}

// NewRSA generates a new RSA key.
func NewRSA(keySize int) (k *RSA, err error) {
	switch keySize {
	case 2048:
	case 4096:
	default:
		return nil, fmt.Errorf("invalid key size %d", keySize)
	}
	var rsaKey *rsa.PrivateKey
	rsaKey, err = rsa.GenerateKey(cryptorand.Reader, keySize)
	if err != nil {
		return nil, err
	}
	return &RSA{privateKey: rsaKey}, nil
}

// LoadRSA loads a RSA key from existing key bytes.
func LoadRSA(privateKeyBytes []byte) (k *RSA, err error) {
	block, _ := pem.Decode(privateKeyBytes)
	if block == nil {
		return nil, errors.New("failed to decode private key bytes")
	}
	var rsaKey *rsa.PrivateKey
	rsaKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &RSA{privateKey: rsaKey}, nil
}

func (k *RSA) PrivateKey() *rsa.PrivateKey {
	return k.privateKey
}

func (k *RSA) PublicKey() *rsa.PublicKey {
	return &k.privateKey.PublicKey
}

func (k *RSA) PrivateKeyBytes() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k.privateKey),
	})
}

func (k *RSA) PublicKeyBytes() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&k.privateKey.PublicKey),
	})
}

// SavePrivateKey saves the private key to the path.
// Give the extension ".key".
func (k *RSA) SavePrivateKey(p string) (err error) {
	if err = os.MkdirAll(filepath.Dir(p), 0600); err != nil {
		return err
	}
	var f *os.File
	f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(k.PrivateKeyBytes())
	return err
}

// SavePublicKey saves the public key to the path.
// Give the extension ".pem".
func (k *RSA) SavePublicKey(p string) (err error) {
	if err = os.MkdirAll(filepath.Dir(p), 0600); err != nil {
		return err
	}
	var f *os.File
	f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(k.PublicKeyBytes())
	return err
}

// EncryptWithPublicKey encrypts data with the public key.
// The only way to decrypt the data is through the corresponding private key.
func (k *RSA) EncryptWithPublicKey(payload []byte) (cipherText []byte, err error) {
	return rsa.EncryptPKCS1v15(cryptorand.Reader, k.PublicKey(), payload)
}

// DecryptWithPrivateKey decrypts the cipher text with the corresponding private key,
// which was encrypted by a public key.
func (k *RSA) DecryptWithPrivateKey(cipherText []byte) (payload []byte, err error) {
	return rsa.DecryptPKCS1v15(cryptorand.Reader, k.privateKey, cipherText)
}

// SignPayloadWithPrivateKey signs data with a private key
// and returns the signature. This signature can be verified
// by the corresponding public key, to prove which private key
// had produced the signature.
func (k *RSA) SignPayloadWithPrivateKey(payload []byte) (sig []byte, err error) {
	opt := &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA512,
	}
	hash := opt.Hash.New()
	if _, err = hash.Write(payload); err != nil {
		return nil, fmt.Errorf("failed to hash %v", err)
	}
	hashedData := hash.Sum(nil)
	sig, err = k.privateKey.Sign(cryptorand.Reader, hashedData, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to sign %v", err)
	}
	return sig, nil
}

// VerifyPayloadWithPublicKey verifies payload with a public key
// and the signature produced by a corresponding private key.
func (k *RSA) VerifyPayloadWithPublicKey(payload []byte, sig []byte) (err error) {
	opt := &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthEqualsHash,
		Hash:       crypto.SHA512,
	}
	hash := opt.Hash.New()
	if _, err = hash.Write(payload); err != nil {
		return fmt.Errorf("failed to hash payload %v", err)
	}
	if err = rsa.VerifyPSS(k.PublicKey(), crypto.SHA512, hash.Sum(nil), sig, opt); err != nil {
		return fmt.Errorf("failed to verify payload %v", err)
	}
	return nil
}

// SignRootCertificate self-signs a new root certificate with its public and private key.
// The root certificate authority (root CA) issues a new certificate
// to the subscriber. The certificate vouches for the binding between
// an existing public key and the name.
func (k *RSA) SignRootCertificate() (err error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	var sn *big.Int
	sn, err = cryptorand.Int(cryptorand.Reader, limit)
	if err != nil {
		return err
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          sn,
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		NotBefore:             now.Add(time.Hour * -48),
		NotAfter:              now.Add(time.Hour * 24 * 365),
		PublicKey:             k.PublicKey(),
	}
	parent := template

	var certBytes []byte
	certBytes, err = x509.CreateCertificate(cryptorand.Reader, &template, &parent, k.PublicKey(), k.privateKey)
	if err != nil {
		return err
	}
	k.rootCert, err = x509.ParseCertificate(certBytes)
	if err != nil {
		return err
	}
	return nil
}

func (k *RSA) RootCertificate() *x509.Certificate {
	return k.rootCert
}

func (k *RSA) RootCertificateBytes() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: k.rootCert.Raw,
	})
}

// SaveRootCertificate saves the root certificate to the path.
// Give the extension ".crt".
func (k *RSA) SaveRootCertificate(p string) (err error) {
	if err = os.MkdirAll(filepath.Dir(p), 0600); err != nil {
		return err
	}
	var f *os.File
	f, err = os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(k.RootCertificateBytes())
	return err
}
