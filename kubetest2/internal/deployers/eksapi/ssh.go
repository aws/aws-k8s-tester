package eksapi

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"

	"k8s.io/klog"
)

func generateSSHKey() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	privateKeyFile := path.Join(home, ".ssh", "id_rsa")
	if _, err := os.Stat(privateKeyFile); errors.Is(err, os.ErrNotExist) {
		klog.V(2).Infof("Generating SSH key: %s", privateKeyFile)
		out, err := exec.Command("ssh-keygen", "-P", "''", "-t", "rsa", "-b", "2048", "-f", privateKeyFile).CombinedOutput()
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	} else if err != nil {
		return err
	}
	return nil
}

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
