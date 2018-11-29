package ingress

import (
	"errors"
	"sort"

	"github.com/aws/aws-k8s-tester/internal/eks/alb/ingress/path"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

// ConfigIngressTestServerIngressSpec defines ingress-test-server Ingress spec configuration.
type ConfigIngressTestServerIngressSpec struct {
	// MetadataName is the name to put in metadata.
	MetadataName string
	// MetadataNamespace is the space to apply ingress to.
	MetadataNamespace string

	// Annotations to configure Ingress and Service resource objects.
	Annotations map[string]string

	// IngressPaths contains additional ingress paths
	// that are manually constructed.
	IngressPaths []v1beta1.HTTPIngressPath

	// GenTargetServiceRoutesN is the number of ingress rule paths to generate.
	GenTargetServiceRoutesN int
	// GenTargetServiceName is the name of target backend service.
	GenTargetServiceName string
	// GenTargetServicePort is the target service port to the backend service.
	GenTargetServicePort int
}

var sampleIngressPath = v1beta1.HTTPIngressPath{
	Path: "",
	Backend: v1beta1.IngressBackend{
		ServiceName: "",
		ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(0)},
	},
}

// CreateIngressTestServerIngressSpec generates Ingress spec.
// Reference https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta.
func CreateIngressTestServerIngressSpec(cfg ConfigIngressTestServerIngressSpec) (string, error) {
	if cfg.MetadataName == "" {
		return "", errors.New("empty MetadataName")
	}
	if cfg.MetadataNamespace == "" {
		return "", errors.New("empty MetadataNamespace")
	}
	if len(cfg.IngressPaths) == 0 && cfg.GenTargetServiceRoutesN == 0 {
		return "", errors.New("empty routes")
	}

	iss := cfg.IngressPaths
	if cfg.GenTargetServiceRoutesN > 0 {
		for i := 0; i < cfg.GenTargetServiceRoutesN; i++ {
			copied := sampleIngressPath
			copied.Path = path.Create(i)
			copied.Backend.ServiceName = cfg.GenTargetServiceName
			copied.Backend.ServicePort.IntVal = int32(cfg.GenTargetServicePort)
			iss = append(iss, copied)
		}
	}
	sort.Sort(ingressPaths(iss))

	ing := v1beta1.Ingress{
		TypeMeta: v1.TypeMeta{
			APIVersion: "extensions/v1beta1",
			Kind:       "Ingress",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:        cfg.MetadataName,
			Namespace:   cfg.MetadataNamespace,
			Annotations: cfg.Annotations,
			Labels: map[string]string{
				"app": cfg.MetadataName,
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: "",
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: iss,
						},
					},
				},
			},
		},
	}
	d, err := yaml.Marshal(ing)
	if err != nil {
		return "", err
	}
	return string(d), nil
}

type ingressPaths []v1beta1.HTTPIngressPath

func (ss ingressPaths) Len() int      { return len(ss) }
func (ss ingressPaths) Swap(i, j int) { ss[i], ss[j] = ss[j], ss[i] }
func (ss ingressPaths) Less(i, j int) bool {
	return ss[i].Path < ss[j].Path
}
