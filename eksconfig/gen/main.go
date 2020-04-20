// gen generates eksconfig documentation.
package main

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/eksconfig"
)

func main() {
	doc := createDoc()
	if err := ioutil.WriteFile("eksconfig/README.md", []byte("\n```\n"+doc+"```\n"), 0666); err != nil {
		panic(err)
	}
	fmt.Println("generated")
}

func createDoc() string {
	b := strings.Builder{}
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefix, &eksconfig.Config{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixParameters, &eksconfig.Parameters{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnNodeGroups, &eksconfig.AddOnNodeGroups{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnManagedNodeGroups, &eksconfig.AddOnManagedNodeGroups{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnNLBHelloWorld, &eksconfig.AddOnNLBHelloWorld{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnALB2048, &eksconfig.AddOnALB2048{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnJobsPi, &eksconfig.AddOnJobsPi{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnJobsEcho, &eksconfig.AddOnJobsEcho{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCronJobs, &eksconfig.AddOnCronJobs{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCSRs, &eksconfig.AddOnCSRs{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnConfigMaps, &eksconfig.AddOnConfigMaps{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnSecrets, &eksconfig.AddOnSecrets{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnIRSA, &eksconfig.AddOnIRSA{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnFargate, &eksconfig.AddOnFargate{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnAppMesh, &eksconfig.AddOnAppMesh{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnWordpress, &eksconfig.AddOnWordpress{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnDashboard, &eksconfig.AddOnDashboard{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnPrometheusGrafana, &eksconfig.AddOnPrometheusGrafana{}))
	b.WriteByte('\n')
	b.WriteString(writeDoc(eksconfig.EnvironmentVariablePrefixAddOnKubeflow, &eksconfig.AddOnKubeflow{}))
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
