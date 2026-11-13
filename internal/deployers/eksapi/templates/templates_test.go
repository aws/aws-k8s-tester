package templates

import (
	"bytes"
	"strings"
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

func Test_UnmanagedNodegroupEnclaveOptions(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		buf := bytes.Buffer{}
		err := UnmanagedNodegroup.Execute(&buf, UnmanagedNodegroupTemplateData{
			InstanceTypes:  []string{"m8i.4xlarge"},
			EnclaveEnabled: enabled,
		})
		if err != nil {
			t.Fatal(err)
		}
		want := "\n        EnclaveOptions:\n          Enabled: true\n        IamInstanceProfile:"
		if got := strings.Contains(buf.String(), want); got != enabled {
			t.Errorf("EnclaveEnabled=%t: rendered EnclaveOptions block present=%t", enabled, got)
		}
	}
}
