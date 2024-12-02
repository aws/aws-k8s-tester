package awssdk

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	"k8s.io/klog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// NewConfig returns an AWS SDK config
// It will panic if the cnfig cannot be created
func NewConfig() aws.Config {
	c, err := config.LoadDefaultConfig(context.TODO(), withCertsEnv())
	if err != nil {
		klog.Fatalf("failed to create AWS SDK config: %v", err)
	}
	return c
}

func withCertsEnv() func(*config.LoadOptions) error {
	return func(lo *config.LoadOptions) error {
		certs := os.Getenv("CERTS_CONTENT")
		if certs != "" {
			klog.Infof("Loading certificates from CERTS_CONTENT")
			lo.HTTPClient = newHTTPClientWithCerts([]byte(certs))
		} else {
			klog.Warningf("CERTS_CONTENT environment variable is not set or empty")
		}
		return nil
	}
}

func newHTTPClientWithCerts(certData []byte) *http.Client {
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(certData); !ok {
		klog.Fatalf("Failed to append provided certificates")
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
	return &http.Client{Transport: transport}
}
