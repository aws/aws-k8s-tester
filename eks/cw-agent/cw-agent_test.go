package cwagent

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

func TestTemplateCWAgentConf(t *testing.T) {
	tr := templateCWAgentConf{
		RegionName:  "us-east-1",
		ClusterName: "test-cluster",
	}
	tpl := template.Must(template.New("TemplateCWAgentConf").Parse(TemplateCWAgentConf))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, tr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"region": "us-east-1"`) {
		t.Fatalf("unexpected region %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"cluster_name": "test-cluster",`) {
		t.Fatalf("unexpected cluster_name %s", buf.String())
	}

	fmt.Println(buf.String())
}
