package eksctl

import (
	"bytes"
	"log"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
)

const configYAMLTemplate = `
---
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig
metadata:
  name: "{{.ClusterName}}"
  region: "{{.Region}}"
  {{- if .KubernetesVersion}}
  version: "{{.KubernetesVersion}}"
  {{- end}}
managedNodeGroups:
  - name: managed
    {{- if .AMI}}
    ami: "{{.AMI}}"
    {{- end}}
    amiFamily: AmazonLinux2
    {{- if .InstanceTypes}}
    instanceTypes:
      {{- range $instanceType := .InstanceTypes}}
      - "{{$instanceType}}"
      {{- end}}
    {{- end}}
	{{- if gt .Nodes 0}}
    minSize: {{.Nodes}}
    maxSize: {{.Nodes}}
    desiredCapacity: {{.Nodes}}
	{{- end}}
	{{- if .AMI}}
    overrideBootstrapCommand: |
      #!/bin/bash
      source /var/lib/cloud/scripts/eksctl/bootstrap.helper.sh
      /etc/eks/bootstrap.sh {{.ClusterName}} --kubelet-extra-args "--node-labels=${NODE_LABELS}"
	{{- end}}
`

type clusterConfigTemplateParams struct {
	UpOptions
	ClusterName string
	Region      string
}

func (d *deployer) RenderClusterConfig() ([]byte, error) {
	templateParams := clusterConfigTemplateParams{
		UpOptions:   *d.UpOptions,
		ClusterName: d.commonOptions.RunID(),
		Region:      aws.StringValue(d.eksClient.Config.Region),
	}
	log.Printf("rendering cluster config template with params: %+v", templateParams)
	t, err := template.New("configYAML").Parse(configYAMLTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, templateParams)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
