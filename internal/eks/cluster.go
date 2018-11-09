package eks

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

// isEKSDeletedGoClient returns true if error from EKS API indicates that
// the EKS cluster has already been deleted.
func isEKSDeletedGoClient(err error) bool {
	if err == nil {
		return false
	}
	/*
	   https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html#AmazonEKS-Type-Cluster-status

	   CREATING
	   ACTIVE
	   DELETING
	   FAILED
	*/
	// ResourceNotFoundException: No cluster found for name: aws-k8s-tester-155468BC717E03B003\n\tstatus code: 404, request id: 1e3fe41c-b878-11e8-adca-b503e0ba731d
	return strings.Contains(err.Error(), "No cluster found for name: ")
}

const kubeConfigTempl = `---
apiVersion: v1
clusters:
- cluster:
    server: {{.ClusterEndpoint}}
    certificate-authority-data: {{.ClusterCA}}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1alpha1
      command: {{ .AWSIAMAuthenticatorPath }}
      args:
        - token
        - -i
        - {{.ClusterName}}

`

type kubeConfig struct {
	AWSIAMAuthenticatorPath string
	ClusterEndpoint         string
	ClusterCA               string
	ClusterName             string
}

func writeKubeConfig(awsIAMAuthenticatorPath, ep, ca, clusterName, p string) (err error) {
	kc := kubeConfig{
		AWSIAMAuthenticatorPath: awsIAMAuthenticatorPath,
		ClusterEndpoint:         ep,
		ClusterCA:               ca,
		ClusterName:             clusterName,
	}
	tpl := template.Must(template.New("kubeCfgTempl").Parse(kubeConfigTempl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, kc); err != nil {
		return err
	}
	os.Setenv("DEFAULT_KUBECONFIG", p)
	return ioutil.WriteFile(p, buf.Bytes(), 0600)
}
