package eksapi

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"text/template"
)

const kubeconfigPerm = 0666

var kubeconfigTemplate = `---
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: {{ .ClusterCertificateAuthority }}
    server: {{ .ClusterEndpoint }}
  name: {{ .ClusterARN }}
contexts:
- context:
    cluster: {{ .ClusterARN }}
    user: {{ .ClusterARN }}
  name: {{ .ClusterARN }}
current-context: {{ .ClusterARN }}
preferences: {}
users:
- name: {{ .ClusterARN }}
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
      - eks
      - get-token
      - --cluster-name
      - {{ .ClusterName }}
`

type kubeconfigTemplateParameters struct {
	ClusterCertificateAuthority string
	ClusterARN                  string
	ClusterEndpoint             string
	ClusterName                 string
}

func writeKubeconfig(cluster *Cluster, kubeconfigPath string) error {
	if cluster == nil {
		return fmt.Errorf("Cluster is nil, you might need set --static-cluster-name or set --up to initial cluster resrouces")
	}
	slog.Info("writing kubeconfig", "path", kubeconfigPath, "clusterArn", cluster.arn)
	templateParams := kubeconfigTemplateParameters{
		ClusterCertificateAuthority: cluster.certificateAuthorityData,
		ClusterARN:                  cluster.arn,
		ClusterEndpoint:             cluster.endpoint,
		ClusterName:                 cluster.name,
	}

	kubeconfig := bytes.Buffer{}

	t, err := template.New("kubeconfig").Parse(kubeconfigTemplate)
	if err != nil {
		return err
	}
	err = t.Execute(&kubeconfig, templateParams)
	if err != nil {
		return err
	}

	err = os.WriteFile(kubeconfigPath, kubeconfig.Bytes(), kubeconfigPerm)
	if err != nil {
		return err
	}

	slog.Info("wrote kubeconfig", "path", kubeconfigPath, "content", kubeconfig.String())
	return nil
}
