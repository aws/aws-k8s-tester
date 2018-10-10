package ingress

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/awstester/internal/eks/ingress/path"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreateIngressTestServerIngressSpec(t *testing.T) {
	cfg1 := ConfigIngressTestServerIngressSpec{
		MetadataName:      "ingress-for-alb-ingress-controller-service",
		MetadataNamespace: "kube-system",
		TargetType:        "instance",
		SubnetIDs: []string{
			"subnet-03e98399a9f14192d",
			"subnet-0b5f7e3100a32cdde",
			"subnet-0da17fa86e91b5943",
		},
		SecurityGroupIDs: []string{
			"sg-046b5c8dc0da4e6b7",
			"sg-0f993f0955cf33720",
		},
		IngressPaths: []v1beta1.HTTPIngressPath{
			{
				Path: "/metrics",
				Backend: v1beta1.IngressBackend{
					ServiceName: "alb-ingress-controller-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
		},
	}
	d1, err := CreateIngressTestServerIngressSpec(cfg1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d1, "alb-ingress-controller-service") {
		t.Fatal("'alb-ingress-controller-service' expected but not found")
	}
	fmt.Println(d1)

	cfg2 := ConfigIngressTestServerIngressSpec{
		MetadataName:      "ingress-for-ingress-test-server-service",
		MetadataNamespace: "default",
		TargetType:        "instance",
		SubnetIDs: []string{
			"subnet-03e98399a9f14192d",
			"subnet-0b5f7e3100a32cdde",
			"subnet-0da17fa86e91b5943",
		},
		SecurityGroupIDs: []string{
			"sg-046b5c8dc0da4e6b7",
			"sg-0f993f0955cf33720",
		},
		IngressPaths: []v1beta1.HTTPIngressPath{
			{
				Path: path.Path,
				Backend: v1beta1.IngressBackend{
					ServiceName: "ingress-test-server-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
			{
				Path: path.PathMetrics,
				Backend: v1beta1.IngressBackend{
					ServiceName: "ingress-test-server-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
		},
		GenTargetServiceName:    "ingress-test-server-service",
		GenTargetServicePort:    80,
		GenTargetServiceRoutesN: 3,
	}
	d2, err := CreateIngressTestServerIngressSpec(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d2, path.PathMetrics) {
		t.Fatalf("%q expected but not found", path.PathMetrics)
	}
	fmt.Println(d2)
}
