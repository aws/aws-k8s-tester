package eks

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/aws/awstester/pkg/fileutil"

	gyaml "github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
// https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
const nodeGroupStackTemplateURL = "https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-nodegroup.yaml"

// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
const configMapNodeAuthTempl = `---
apiVersion: v1
kind: ConfigMap

metadata:
  name: aws-auth
  namespace: kube-system

data:
  mapRoles: |
    - rolearn: {{.WorkerNodeInstanceRoleARN}}
      %s
      groups:
      - system:bootstrappers
      - system:nodes

`

type configMapNodeAuth struct {
	WorkerNodeInstanceRoleARN string
}

func writeConfigMapNodeAuth(arn string) (p string, err error) {
	kc := configMapNodeAuth{WorkerNodeInstanceRoleARN: arn}
	tpl := template.Must(template.New("configMapNodeAuthTempl").Parse(configMapNodeAuthTempl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, kc); err != nil {
		return "", err
	}
	// avoid '{{' conflicts with Go
	txt := fmt.Sprintf(buf.String(), `username: system:node:{{EC2PrivateDNSName}}`)
	return fileutil.WriteTempFile([]byte(txt))
}

/*
expects:

NAME                                           STATUS    ROLES     AGE       VERSION
ip-192-168-192-77.us-west-2.compute.internal   Ready     <none>    2d        v1.10.3
ip-192-168-87-77.us-west-2.compute.internal    Ready     <none>    2d        v1.10.3
*/
func countReadyNodesFromKubectlGetNodesOutputSimple(kubectlOutput []byte) int {
	s := strings.Replace(string(kubectlOutput), "NotReady", "N.o.t.R.e.a.d.y", -1)
	return strings.Count(s, "Ready")
}

type nodeConditions struct {
	Conditions []corev1.NodeCondition `json:"conditions"`
}

// TODO: use k8s.io/client-go to list nodes

func countReadyNodesFromKubectlGetNodesOutputYAML(kubectlOutput []byte) (int, error) {
	ls := new(unstructured.UnstructuredList)
	if err := gyaml.Unmarshal(kubectlOutput, ls); err != nil {
		return 0, err
	}
	cnt := 0
	for _, item := range ls.Items {
		if item.GetKind() != "Node" {
			return 0, fmt.Errorf("unexpected item type %q", item.GetKind())
		}
		sm, ok := item.UnstructuredContent()["status"]
		if !ok {
			return 0, fmt.Errorf("'status' key not found at %v", item)
		}
		mm, ok := sm.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("expected map[string]interface{}, got %v", reflect.TypeOf(sm))
		}
		d, err := gyaml.Marshal(mm)
		if err != nil {
			return 0, fmt.Errorf("failed to parse node statuses (%v)", err)
		}
		ss := new(nodeConditions)
		if err = gyaml.Unmarshal(d, ss); err != nil {
			return 0, fmt.Errorf("failed to unmarshal node statuses (%v)", err)
		}
	done:
		for _, cond := range ss.Conditions {
			if cond.Reason == "KubeletReady" && cond.Type == corev1.NodeReady {
				cnt++
				break done
			}
		}
	}
	return cnt, nil
}
