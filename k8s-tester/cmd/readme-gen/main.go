// gen generates eksconfig documentation.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester"
	cloudwatch_agent "github.com/aws/aws-k8s-tester/k8s-tester/cloudwatch-agent"
	"github.com/olekukonko/tablewriter"
)

func main() {
	doc := createDoc()
	if err := ioutil.WriteFile("../../README.config.md", []byte("\n```\n"+doc+"```\n"), 0666); err != nil {
		panic(err)
	}
	fmt.Println("generated")
}

func createDoc() string {
	es := &enableEnvVars{}
	b := strings.Builder{}

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(k8s_tester.ENV_PREFIX, &k8s_tester.Config{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(k8s_tester.ENV_PREFIX+cloudwatch_agent.Env()+"_", &cloudwatch_agent.Config{}))

	b.WriteByte('\n')
	b.WriteByte('\n')

	txt := b.String()

	return txt
}

type enableEnvVars struct {
}

var columns = []string{
	"environmental variable",
	"read only",
	"type",
	"go type",
}

func (es *enableEnvVars) writeDoc(pfx string, st interface{}) string {
	buf := bytes.NewBuffer(nil)
	tb := tablewriter.NewWriter(buf)
	tb.SetAutoWrapText(false)
	tb.SetAlignment(tablewriter.ALIGN_LEFT)
	tb.SetColWidth(1500)
	tb.SetCenterSeparator("*")
	tb.SetHeader(columns)

	ts := reflect.TypeOf(st)
	tp, vv := reflect.TypeOf(st).Elem(), reflect.ValueOf(st).Elem()
	for i := 0; i < tp.NumField(); i++ {
		jv := tp.Field(i).Tag.Get("json")
		if jv == "" {
			continue
		}

		if jv == "-" {
			continue
		}

		pointer := "false"
		if vv.Field(i).Type().Kind() == reflect.Ptr {
			pointer = "true"
		}

		readOnly := "false"
		if tp.Field(i).Tag.Get("read-only") == "true" {
			readOnly = "true"
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv

		tb.Append([]string{
			env,
			fmt.Sprintf("read-only %q, add-on %q", readOnly, pointer),
			fmt.Sprintf("%s.%s", ts, tp.Field(i).Name),
			fmt.Sprintf("%s", vv.Field(i).Type()),
		})
	}

	tb.Render()
	return buf.String()
}
