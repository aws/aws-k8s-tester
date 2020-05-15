// gen generates ec2config documentation.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/olekukonko/tablewriter"
)

func main() {
	doc := createDoc()
	if err := ioutil.WriteFile("ec2config/README.md", []byte("\n```\n"+doc+"```\n"), 0666); err != nil {
		panic(err)
	}
	fmt.Println("generated")
}

func createDoc() string {
	b := strings.Builder{}
	b.WriteString(writeDoc(ec2config.AWS_K8S_TESTER_EC2_PREFIX, &ec2config.Config{}))
	return b.String()
}

var columns = []string{
	"environmental variable",
	"read only",
	"type",
	"go type",
}

func writeDoc(pfx string, st interface{}) string {
	buf := bytes.NewBuffer(nil)
	tb := tablewriter.NewWriter(buf)
	tb.SetAutoWrapText(false)
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
		if vv.Field(i).Type().Kind() == reflect.Ptr {
			continue
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
			fmt.Sprintf("read-only %q", readOnly),
			fmt.Sprintf("%s.%s", ts, tp.Field(i).Name),
			fmt.Sprintf("%s", vv.Field(i).Type()),
		})
	}

	tb.SetAlignment(tablewriter.ALIGN_CENTER)
	tb.Render()
	return buf.String()
}

