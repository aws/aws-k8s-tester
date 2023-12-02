package eksapi

import (
	"bytes"
	"context"
	"os"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"k8s.io/klog"
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
	ClusterCertificateAuthority *string
	ClusterARN                  *string
	ClusterEndpoint             *string
	ClusterName                 *string
}

func writeKubeconfig(eksClient *eks.Client, clusterName string, kubeconfigPath string) error {
	klog.Infof("writing kubeconfig to %s for cluster: %s", kubeconfigPath, clusterName)
	out, err := eksClient.DescribeCluster(context.TODO(), &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return err
	}
	templateParams := kubeconfigTemplateParameters{
		ClusterCertificateAuthority: out.Cluster.CertificateAuthority.Data,
		ClusterARN:                  out.Cluster.Arn,
		ClusterEndpoint:             out.Cluster.Endpoint,
		ClusterName:                 out.Cluster.Name,
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

	klog.Infof("wrote kubeconfig: %s\n%s", kubeconfigPath, kubeconfig.String())
	return nil
}
