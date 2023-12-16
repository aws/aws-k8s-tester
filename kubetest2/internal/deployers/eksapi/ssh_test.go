package eksapi

import (
	"path"
	"testing"
)

func Test_generateSSHKey(t *testing.T) {
	tmp := t.TempDir()
	privateKeyPath := path.Join(tmp, ".ssh", "id_rsa")
	publicKeyPath := privateKeyPath + ".pub"
	if err := generateSSHKeyToFile(privateKeyPath, publicKeyPath); err != nil {
		t.Fatal(err)
	}
}
