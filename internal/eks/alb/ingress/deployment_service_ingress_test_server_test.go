package ingress

import (
	"fmt"
	"strings"
	"testing"
)

func TestCreateDeploymentServiceIngressTestServer(t *testing.T) {
	cfg := ConfigDeploymentServiceIngressTestServer{
		Name:         "ingress-test-server",
		ServiceName:  "ingress-test-server-service",
		Namespace:    "default",
		Image:        "607362164682.dkr.ecr.us-west-2.amazonaws.com/awstester",
		Replicas:     1,
		Routes:       10,
		ResponseSize: 10,
	}
	d, err := CreateDeploymentServiceIngressTestServer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "--response-size=10") {
		t.Fatalf("expected '--response-size=10', got %q", d)
	}
	if !strings.Contains(d, "--routes=10") {
		t.Fatalf("expected '--routes=10', got %q", d)
	}
	fmt.Println(d)
}
