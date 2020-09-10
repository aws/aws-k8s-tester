package mng

import (
	"bytes"
	"strings"
	"testing"
	"text/template"
)

func TestTemplateMNG(t *testing.T) {
	tpl := template.Must(template.New("TemplateMNG").Parse(TemplateMNG))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateMNG{
		ASGDesiredCapacity:      2,
		ParameterReleaseVersion: parametersReleaseVersion,
		PropertyReleaseVersion:  propertyReleaseVersion,
	}); err != nil {
		t.Fatal(err)
	}
	tmpBody := buf.String()
	if !strings.Contains(tmpBody, `DesiredSize: !Ref ASGDesiredCapacity`) {
		t.Fatalf("expected 'DesiredSize' field, but not found\n%s\n", tmpBody)
	}
}
