/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
)

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

// TODO: use k8s.io/client-go to list nodes

// reference: https://github.com/kubernetes/test-infra/blob/master/kubetest/kubernetes.go

// kubectlGetNodes lists nodes by executing kubectl get nodes, parsing the output into a nodeList object
func kubectlGetNodes(out []byte) (*nodeList, error) {
	nodes := &nodeList{}
	if err := json.Unmarshal(out, nodes); err != nil {
		return nil, fmt.Errorf("error parsing kubectl get nodes output: %v", err)
	}
	return nodes, nil
}

// isReady checks if the node has a Ready Condition that is True
func isReady(node *node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// countReadyNodes returns the number of nodes that have isReady == true
func countReadyNodes(nodes *nodeList) int {
	var ns []*node
	for i := range nodes.Items {
		nd := &nodes.Items[i]
		if isReady(nd) {
			ns = append(ns, nd)
		}
	}
	return len(ns)
}

// nodeList is a simplified version of the v1.NodeList API type
type nodeList struct {
	Items []node `json:"items"`
}

// node is a simplified version of the v1.Node API type
type node struct {
	Metadata metadata   `json:"metadata"`
	Status   nodeStatus `json:"status"`
}

// nodeStatus is a simplified version of the v1.NodeStatus API type
type nodeStatus struct {
	Addresses  []nodeAddress   `json:"addresses"`
	Conditions []nodeCondition `json:"conditions"`
}

// nodeAddress is a simplified version of the v1.NodeAddress API type
type nodeAddress struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// nodeCondition is a simplified version of the v1.NodeCondition API type
type nodeCondition struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Type    string `json:"type"`
}

// metadata is a simplified version of the kubernetes metadata types
type metadata struct {
	Name string `json:"name"`
}
