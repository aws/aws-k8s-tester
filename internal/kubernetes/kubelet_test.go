package kubernetes

import (
	"os"
	"testing"

	"github.com/aws/aws-k8s-tester/internal/pki"
)

func Test_writeKubeletKubeConfigFile(t *testing.T) {
	rsa, err := pki.NewRSA(2048)
	if err != nil {
		t.Fatal(err)
	}
	if err = rsa.SignRootCertificate(); err != nil {
		t.Fatal(err)
	}
	var p string
	p, err = writeKubeletKubeConfigFile(rsa.PrivateKeyBytes(), rsa.PublicKeyBytes(), rsa.RootCertificateBytes())
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(p)
}
