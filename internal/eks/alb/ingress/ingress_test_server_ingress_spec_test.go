package ingress

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress/path"

	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreateIngressTestServerIngressSpec(t *testing.T) {
	cfg1 := ConfigIngressTestServerIngressSpec{
		MetadataName:      "ingress-for-alb-ingress-controller-service",
		MetadataNamespace: "kube-system",

		Annotations: map[string]string{
			"alb.ingress.kubernetes.io/scheme":       "internet-facing",
			"alb.ingress.kubernetes.io/target-type":  "instance",
			"alb.ingress.kubernetes.io/listen-ports": `[{"HTTP":80,"HTTPS": 443}]`,
			"alb.ingress.kubernetes.io/subnets":      "a,b,c",

			// TODO: support SSL
			// alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:us-west-2:220355219862:certificate/fd0beb0d-2a1e-40e7-af04-e1354ea14143
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

		Annotations: map[string]string{
			"alb.ingress.kubernetes.io/scheme":       "internet-facing",
			"alb.ingress.kubernetes.io/target-type":  "instance",
			"alb.ingress.kubernetes.io/listen-ports": `[{"HTTP":80,"HTTPS": 443}]`,
			"alb.ingress.kubernetes.io/subnets":      "a,b,c",

			// TODO: support SSL
			// alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:us-west-2:220355219862:certificate/fd0beb0d-2a1e-40e7-af04-e1354ea14143
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
