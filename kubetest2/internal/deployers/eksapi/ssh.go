package eksapi

import (
	"errors"
	"fmt"
	"os"
	"path"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
)

const sshKeyBits = 2048

func loadSSHPublicKey() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	publicKeyFile := path.Join(home, ".ssh", "id_rsa.pub")
	material, err := os.ReadFile(publicKeyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("SSH public key does not exist: %s", publicKeyFile)
		}
		return "", err
	}
	return string(material), err
}

func generateSSHKey() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	privateKeyFile := path.Join(home, ".ssh", "id_rsa")
	publicKeyFile := privateKeyFile + ".pub"
	if err := generateSSHKeyToFile(privateKeyFile, publicKeyFile); err != nil {
		return fmt.Errorf("failed to generate ssh key: %v", err)
	}
	return nil
}

func generateSSHKeyToFile(privateKeyPath string, publicKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := os.Stat(publicKeyPath); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	klog.Infof("Generating SSH key: %s", privateKeyPath)
	privateKey, err := generatePrivateKey(sshKeyBits)
	if err != nil {
		return err
	}
	publicKeyBytes, err := encodePublicKey(privateKey)
	if err != nil {
		return err
	}
	privateKeyBytes := encodePrivateKeyToPEM(privateKey)
	keyDir := path.Dir(privateKeyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("failed to create directory for SSH key: %v", err)
	}
	if err := os.WriteFile(privateKeyPath, privateKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write SSH private key to %s: %v", privateKeyPath, err)
	}
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0600); err != nil {
		return fmt.Errorf("failed to write SSH public key to %s: %v", publicKeyPath, err)
	}
	return nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}
	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func encodePrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privDER := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDER,
	}
	return pem.EncodeToMemory(&privBlock)
}

func encodePublicKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)
	return publicKeyBytes, nil
}
