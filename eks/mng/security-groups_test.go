package mng

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateSG(t *testing.T) {
	tpl := template.Must(template.New("TemplateSG").Parse(TemplateSG))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateSG{
		InternetIngressFromPort: 0,
		InternetIngressToPort:   32767,
	}); err != nil {
		t.Fatal(err)
	}
	tmpl := buf.String()

	fmt.Println(tmpl)
}
