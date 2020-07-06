package fluentd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"text/template"
)

func TestTemplateFluentdConf(t *testing.T) {
	tr := templateFluentdConf{
		Threads:                       10,
		MetadataLogLevel:              "info",
		MetadataCacheSize:             100,
		MetadataWatch:                 true,
		MetadataSkipLabels:            false,
		MetadataSkipMasterURL:         true,
		MetadataSkipContainerMetadata: false,
		MetadataSkipNamespaceMetadata: true,
	}
	tpl := template.Must(template.New("TemplateFluentdConf").Parse(TemplateFluentdConf))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, tr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `@log_level "info"`) {
		t.Fatalf("expected log level 'info', got %s", buf.String())
	}
	if !strings.Contains(buf.String(), `cache_size 100`) {
		t.Fatalf("expected cache size '1000', got %s", buf.String())
	}

	fmt.Println(buf.String())
}
