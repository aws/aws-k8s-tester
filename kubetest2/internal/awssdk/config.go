package awssdk

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"k8s.io/klog"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// NewConfig returns an AWS SDK config
// It will panic if the config cannot be created
func NewConfig(optFns ...func(*config.LoadOptions) error) aws.Config {
	c, err := config.LoadDefaultConfig(
		context.TODO(),
		optFns...,
	)
	if err != nil {
		klog.Fatalf("failed to create AWS SDK config: %v", err)
	}
	return c
}

// WithCertsPath loads certificates from a file path
func WithCertsPath(certsPath string) func(*config.LoadOptions) error {
	return func(lo *config.LoadOptions) error {
		if certsPath != "" {
			klog.Infof("Loading certificates from file: %s", certsPath)
			certData, err := ioutil.ReadFile(certsPath)
			if err != nil {
				klog.Fatalf("Failed to read certificates from file: %v", err)
				return err
			}
			klog.Infof("Certificates loaded from file")
			lo.HTTPClient = newHTTPClientWithCerts(certData)
		}
		return nil
	}
}

func newHTTPClientWithCerts(certData []byte) *http.Client {
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certData)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
	return &http.Client{Transport: transport}
}
