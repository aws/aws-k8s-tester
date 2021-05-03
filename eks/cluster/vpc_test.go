package cluster

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

func TestTemplateVPCPublicPrivate(t *testing.T) {
	tpl := template.Must(template.New("TemplateVPCPublicPrivate").Parse(TemplateVPCPublicPrivate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, templateVPCPublicPrivate{}); err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())

	buf.Reset()
	if err := tpl.Execute(buf, templateVPCPublicPrivate{
		IsIsolated: true,
	}); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(buf.String(), "Value: !Sub '${AWS::StackName}-EIP2") {
		t.Fail()
	}

	fmt.Println(buf.String())
}
