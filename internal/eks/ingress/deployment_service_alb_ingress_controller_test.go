package ingress

import (
	"fmt"
	"strings"
	"testing"
)

func TestCreateDeploymentServiceALBIngressController(t *testing.T) {
	cfg := ConfigDeploymentServiceALBIngressController{
		AWSRegion:   "us-west-2",
		Name:        "alb-ingress-controller",
		ServiceName: "alb-ingress-controller-service",
		Namespace:   "kube-system",
		Image:       "quay.io/coreos/alb-ingress-controller:1.0-beta.7",
		ClusterName: "EKS-PROW-CLUSTER",
	}
	d, err := CreateDeploymentServiceALBIngressController(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "--cluster-name=EKS-PROW-CLUSTER") {
		t.Fatalf("expected '--cluster-name=EKS-PROW-CLUSTER', got %q", d)
	}
	fmt.Println(d)
}
