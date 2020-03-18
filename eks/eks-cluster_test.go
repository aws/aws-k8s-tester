package eks

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateEKSCluster(t *testing.T) {
	tpl := template.Must(template.New("TemplateEKSCluster").Parse(TemplateEKSCluster))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateEKSCluster{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())

	buf.Reset()
	if err := tpl.Execute(buf, templateEKSCluster{
		AWSEncryptionProviderCMKARN: "aaaaa",
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())
}
