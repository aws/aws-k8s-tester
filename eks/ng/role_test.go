package ng

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateRole(t *testing.T) {
	tpl := template.Must(template.New("TemplateRole").Parse(TemplateRole))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateRole{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())

	buf.Reset()
	if err := tpl.Execute(buf, templateRole{
		S3BucketName:  "hello",
		ClusterName:   "test-cluster",
		ASGPolicyData: asgPolicyData,
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())
}
