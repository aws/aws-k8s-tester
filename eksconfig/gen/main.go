// gen generates eksconfig documentation.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/olekukonko/tablewriter"
)

func main() {
	doc := createDoc()
	if err := ioutil.WriteFile("eksconfig/README.md", []byte("\n```\n"+doc+"```\n"), 0666); err != nil {
		panic(err)
	}
	fmt.Println("generated")
}

func createDoc() string {
	es := &enableEnvVars{envs: make([]string, 0)}
	b := strings.Builder{}

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_PREFIX, &eksconfig.Config{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ENCRYPTION_PREFIX, &eksconfig.Encryption{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ROLE_PREFIX, &eksconfig.Role{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_VPC_PREFIX, &eksconfig.VPC{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_CNI_VPC_PREFIX, &eksconfig.AddOnCNIVPC{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_PREFIX, &eksconfig.AddOnNodeGroups{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_PREFIX+"ROLE_", &eksconfig.Role{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_PREFIX, &eksconfig.AddOnManagedNodeGroups{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_PREFIX+"ROLE_", &eksconfig.Role{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_CW_AGENT_PREFIX, &eksconfig.AddOnCWAgent{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.AWS_K8S_TESTER_EKS_ADD_ON_FLUENTD_PREFIX, &eksconfig.AddOnFluentd{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnMetricsServer, &eksconfig.AddOnMetricsServer{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnConformance, &eksconfig.AddOnConformance{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnAppMesh, &eksconfig.AddOnAppMesh{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCSIEBS, &eksconfig.AddOnCSIEBS{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnKubernetesDashboard, &eksconfig.AddOnKubernetesDashboard{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnPrometheusGrafana, &eksconfig.AddOnPrometheusGrafana{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnPHPApache, &eksconfig.AddOnPHPApache{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnNLBHelloWorld, &eksconfig.AddOnNLBHelloWorld{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnNLBGuestbook, &eksconfig.AddOnNLBGuestbook{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnALB2048, &eksconfig.AddOnALB2048{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnJobsPi, &eksconfig.AddOnJobsPi{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnJobsEcho, &eksconfig.AddOnJobsEcho{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCronJobs, &eksconfig.AddOnCronJobs{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCSRsLocal, &eksconfig.AddOnCSRsLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCSRsRemote, &eksconfig.AddOnCSRsRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnConfigmapsLocal, &eksconfig.AddOnConfigmapsLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnConfigmapsRemote, &eksconfig.AddOnConfigmapsRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnSecretsLocal, &eksconfig.AddOnSecretsLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnSecretsRemote, &eksconfig.AddOnSecretsRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnFargate, &eksconfig.AddOnFargate{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnIRSA, &eksconfig.AddOnIRSA{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnIRSAFargate, &eksconfig.AddOnIRSAFargate{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnWordpress, &eksconfig.AddOnWordpress{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnJupyterHub, &eksconfig.AddOnJupyterHub{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString("# NOT WORKING...")
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnKubeflow, &eksconfig.AddOnKubeflow{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnCUDAVectorAdd, &eksconfig.AddOnCUDAVectorAdd{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnClusterLoaderLocal, &eksconfig.AddOnClusterLoaderLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnClusterLoaderRemote, &eksconfig.AddOnClusterLoaderRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnHollowNodesLocal, &eksconfig.AddOnHollowNodesLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnHollowNodesRemote, &eksconfig.AddOnHollowNodesRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnStresserLocal, &eksconfig.AddOnStresserLocal{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnStresserRemote, &eksconfig.AddOnStresserRemote{}))

	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(es.writeDoc(eksconfig.EnvironmentVariablePrefixAddOnClusterVersionUpgrade, &eksconfig.AddOnClusterVersionUpgrade{}))

	b.WriteByte('\n')
	b.WriteByte('\n')

	txt := b.String()

	return fmt.Sprintf("# total %d add-ons\n", len(es.envs)) +
		"# set the following *_ENABLE env vars to enable add-ons, rest are set with default values\n" +
		strings.Join(es.envs, "\n") +
		"\n\n" +
		txt
}

type enableEnvVars struct {
	envs []string
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

		if strings.HasSuffix(env, "_ENABLE") {
			es.envs = append(es.envs, env+"=true \\")
		}
	}

	tb.Render()
	return buf.String()
}
