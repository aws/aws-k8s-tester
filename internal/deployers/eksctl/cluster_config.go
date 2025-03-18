package eksctl

import (
	"bytes"
	"log"
	"text/template"
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
{{- if .WithOIDC}}
iam:
  withOIDC: true
{{- end}}

{{- if .UseUnmanagedNodegroup}}
nodeGroups:
  - name: {{if .NodegroupName}}"{{.NodegroupName}}"{{else}}"ng-1"{{end}}
    {{- if .AMI}}
    ami: "{{.AMI}}"
    {{- end}}
    {{- if .AMIFamily}}
    amiFamily: {{.AMIFamily}}
    {{- else}}
    amiFamily: AmazonLinux2
    {{- end}}
    {{- if .InstanceTypes}}
    instanceType: "{{index .InstanceTypes 0}}"
    {{- end}}
    {{- if gt .Nodes 0}}
    minSize: {{.Nodes}}
    maxSize: {{.Nodes}}
    desiredCapacity: {{.Nodes}}
    {{- end}}
    {{- if .VolumeSize}}
    volumeSize: {{.VolumeSize}}
    {{- end}}
    {{- if .PrivateNetworking}}
    privateNetworking: true
    {{- end}}
    {{- if .AvailabilityZones}}
    availabilityZones:
    {{- range $az := .AvailabilityZones}}
    - "{{$az}}"
    {{- end}}
    {{- end}}
    {{- if and .AMI (eq .AMIFamily "AmazonLinux2")}}
    overrideBootstrapCommand: |
      #!/bin/bash
      source /var/lib/cloud/scripts/eksctl/bootstrap.helper.sh
      /etc/eks/bootstrap.sh {{.ClusterName}} --kubelet-extra-args "--node-labels=${NODE_LABELS}"
    {{- end}}
{{- else}}
managedNodeGroups:
  - name: {{if .NodegroupName}}"{{.NodegroupName}}"{{else}}"managed"{{end}}
    {{- if .AMI}}
    ami: "{{.AMI}}"
    {{- end}}
    {{- if .AMIFamily}}
    amiFamily: {{.AMIFamily}}
    {{- else}}
    amiFamily: AmazonLinux2
    {{- end}}
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
    {{- if .VolumeSize}}
    volumeSize: {{.VolumeSize}}
    {{- end}}
    {{- if .PrivateNetworking}}
    privateNetworking: true
    {{- end}}
    {{- if .EFAEnabled}}
    efaEnabled: true
    {{- end}}
    {{- if .AvailabilityZones}}
    availabilityZones:
    {{- range $az := .AvailabilityZones}}
    - "{{$az}}"
    {{- end}}
    {{- end}}
    {{- if and .AMI (eq .AMIFamily "AmazonLinux2")}}
    overrideBootstrapCommand: |
      #!/bin/bash
      source /var/lib/cloud/scripts/eksctl/bootstrap.helper.sh
      /etc/eks/bootstrap.sh {{.ClusterName}} --kubelet-extra-args "--node-labels=${NODE_LABELS}"
    {{- end}}
{{- end}}
`

type clusterConfigTemplateParams struct {
	UpOptions
	ClusterName string
	Region      string
}

func (d *deployer) RenderClusterConfig() ([]byte, error) {
	d.initClusterName()

	templateParams := clusterConfigTemplateParams{
		UpOptions:   *d.UpOptions,
		ClusterName: d.clusterName,
		Region:      d.awsConfig.Region,
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
