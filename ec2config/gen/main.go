package main

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
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
	b.WriteString(writeDoc(ec2config.EnvironmentVariablePrefix, &ec2config.Config{}))
	return b.String()
}

func writeDoc(pfx string, st interface{}) string {
	var b strings.Builder
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
		rv := "false"
		if tp.Field(i).Tag.Get("read-only") == "true" {
			rv = "true"
		}
		jv = strings.Replace(jv, ",omitempty", "", -1)
		jv = strings.ToUpper(strings.Replace(jv, "-", "_", -1))
		env := pfx + jv
		b.WriteString(fmt.Sprintf(
			"%s | %s.%s | %s | read-only %q\n",
			env,
			ts,
			tp.Field(i).Name,
			vv.Field(i).Type(),
			rv,
		))
	}
	return b.String()
}
