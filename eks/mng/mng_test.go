package mng

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"
)

func TestTemplateMNG(t *testing.T) {
	tpl := template.Must(template.New("TemplateMNG").Parse(TemplateMNG))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateMNG{
		ParameterReleaseVersion: parametersReleaseVersion,
		PropertyReleaseVersion:  propertyReleaseVersion,
	}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())
}
