package templates

import (
	"bytes"
	"testing"
)

func Test_UnmanagedNodegroup(t *testing.T) {
	buf := bytes.Buffer{}
	err := UnmanagedNodegroup.Execute(&buf, UnmanagedNodegroupTemplateData{
		KubernetesVersion: "1.28",
		InstanceTypes: []string{
			"t2.medium",
			"t2.large",
			"t2.xlarge",
		},
	})
	if err != nil {
		t.Error(err)
	}
}
