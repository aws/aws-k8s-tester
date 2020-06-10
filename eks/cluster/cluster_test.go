package cluster

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateCluster(t *testing.T) {
	tpl := template.Must(template.New("TemplateCluster").Parse(TemplateCluster))
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
