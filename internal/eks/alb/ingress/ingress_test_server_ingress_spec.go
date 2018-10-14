package ingress

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/awstester/internal/eks/alb/ingress/path"

	gyaml "github.com/ghodss/yaml"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ConfigIngressTestServerIngressSpec defines ingress-test-server Ingress spec configuration.
type ConfigIngressTestServerIngressSpec struct {
	// MetadataName is the name to put in metadata.
	MetadataName string
	// MetadataNamespace is the space to apply ingress to.
	MetadataNamespace string

	// LogAccess is non-empty to enable ALB access logs.
	LogAccess string
	// Tags is the tags to be added to ingress annotations.
	Tags map[string]string

	// TargetType specifies the target type for target groups:
	// - 'instance' to use node port
	// - 'ip' to use pod IP
	TargetType string

	// SubnetIDs is the list of subnet IDs for EKS control plane VPC stack.
	SubnetIDs []string
	// SecurityGroupIDs is the list of security group IDs for ALB with HTTP/HTTPS wide open.
	// One is from EKS control plane VPC stack.
	// The other is a new one with 80 and 443 TCP ports open.
	SecurityGroupIDs []string

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
	if cfg.TargetType != "instance" && cfg.TargetType != "ip" {
		return "", fmt.Errorf("unknown target type %q", cfg.TargetType)
	}
	if len(cfg.SubnetIDs) == 0 {
		return "", errors.New("empty SubnetIDs")
	}
	if cfg.TargetType == "instance" && len(cfg.SecurityGroupIDs) == 0 {
		return "", errors.New("empty SecurityGroupIDs for target type 'instance'")
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
			Name:      cfg.MetadataName,
			Namespace: cfg.MetadataNamespace,
			Annotations: map[string]string{
				"alb.ingress.kubernetes.io/scheme":       "internet-facing",
				"alb.ingress.kubernetes.io/target-type":  cfg.TargetType,
				"alb.ingress.kubernetes.io/listen-ports": `[{"HTTP":80,"HTTPS": 443}]`,
				"alb.ingress.kubernetes.io/subnets":      strings.Join(cfg.SubnetIDs, ","),

				// TODO: support SSL
				// alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:us-west-2:220355219862:certificate/fd0beb0d-2a1e-40e7-af04-e1354ea14143
			},
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

	if cfg.LogAccess != "" {
		ing.ObjectMeta.Annotations["alb.ingress.kubernetes.io/load-balancer-attributes"] = cfg.LogAccess
	}
	if cfg.TargetType == "instance" {
		ing.ObjectMeta.Annotations["alb.ingress.kubernetes.io/security-groups"] = strings.Join(cfg.SecurityGroupIDs, ",")
	}

	if len(cfg.Tags) > 0 {
		// e.g. alb.ingress.kubernetes.io/tags: Environment=dev,Team=test
		ss := []string{}
		for k, v := range cfg.Tags {
			ss = append(ss, fmt.Sprintf("%s=%s", k, v))
		}
		ing.ObjectMeta.Annotations["alb.ingress.kubernetes.io/tags"] = strings.Join(ss, ",")
	}

	d, err := gyaml.Marshal(ing)
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
