package ng

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateASG(t *testing.T) {
	tpl := template.Must(template.New("TemplateASG").Parse(TemplateASG))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateASG{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())

	buf.Reset()
	if err := tpl.Execute(buf, templateASG{
		Metadata: installSSMAL2Metadata,
		UserData: installSSMAL2UserData,
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())
}
