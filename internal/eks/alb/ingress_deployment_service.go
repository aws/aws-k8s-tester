package alb

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type podConditions struct {
	Conditions []corev1.PodCondition `json:"conditions"`
}

func findReadyPodsFromKubectlGetPodsOutputYAML(kubectlOutput []byte, podPrefix string) bool {
	ls := new(unstructured.UnstructuredList)
	if err := yaml.Unmarshal(kubectlOutput, ls); err != nil {
		return false
	}
	for _, item := range ls.Items {
		if item.GetKind() != "Pod" {
			return false
		}
		sm, ok := item.UnstructuredContent()["metadata"]
		if !ok {
			return false
		}
		mm, ok := sm.(map[string]interface{})
		if !ok {
			return false
		}
		gv, ok := mm["generateName"]
		if !ok {
			continue
		}
		gvv, ok := gv.(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(gvv, podPrefix) {
			continue
		}
		sm, ok = item.UnstructuredContent()["status"]
		if !ok {
			return false
		}
		mm, ok = sm.(map[string]interface{})
		if !ok {
			return false
		}
		d, err := yaml.Marshal(mm)
		if err != nil {
			return false
		}
		ss := new(podConditions)
		if err = yaml.Unmarshal(d, ss); err != nil {
			return false
		}
		for _, cond := range ss.Conditions {
			if cond.Status == corev1.ConditionTrue && cond.Type == corev1.PodReady {
				return true
			}
		}
	}
	return true
}
