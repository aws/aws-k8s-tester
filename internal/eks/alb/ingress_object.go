package alb

import (
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func getHostnameFromKubectlGetIngressOutput(kubectlOutput []byte, serviceName string) string {
	ls := new(unstructured.UnstructuredList)
	if err := yaml.Unmarshal(kubectlOutput, ls); err != nil {
		return "*"
	}
	for _, item := range ls.Items {
		if item.GetKind() != "Ingress" {
			return "*"
		}
		sm, ok := item.UnstructuredContent()["spec"]
		if !ok {
			return "*"
		}
		mm, ok := sm.(map[string]interface{})
		if !ok {
			return "*"
		}
		d, err := yaml.Marshal(mm)
		if err != nil {
			return "*"
		}
		ss := new(v1beta1.IngressSpec)
		if err = yaml.Unmarshal(d, ss); err != nil {
			return "*"
		}
		found := false
		for _, rule := range ss.Rules {
			for _, p := range rule.HTTP.Paths {
				if p.Backend.ServiceName == serviceName {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			continue
		}
		svv, ok := item.UnstructuredContent()["status"]
		if !ok {
			return "*"
		}
		d, err = yaml.Marshal(svv)
		if err != nil {
			return "*"
		}
		st := new(v1.ServiceStatus)
		if err = yaml.Unmarshal(d, st); err != nil {
			return "*"
		}
		if len(st.LoadBalancer.Ingress) < 1 {
			return "*"
		}
		if st.LoadBalancer.Ingress[0].Hostname != "" && st.LoadBalancer.Ingress[0].Hostname != "*" {
			return st.LoadBalancer.Ingress[0].Hostname
		}
	}
	return "*"
}
