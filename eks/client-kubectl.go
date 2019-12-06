package eks

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html
// https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
// aws eks update-kubeconfig --name --role-arn --kubeconfig
func (ts *Tester) updateKUBECONFIG() error {
	if ts.cfg.AWSIAMAuthenticatorPath != "" && ts.cfg.AWSIAMAuthenticatorDownloadURL != "" {
		tpl := template.Must(template.New("tmplKUBECONFIG").Parse(tmplKUBECONFIG))
		buf := bytes.NewBuffer(nil)
		if err := tpl.Execute(buf, kubeconfig{
			ClusterAPIServerEndpoint: ts.cfg.Status.ClusterAPIServerEndpoint,
			ClusterCA:                ts.cfg.Status.ClusterCA,
			AWSIAMAuthenticatorPath:  ts.cfg.AWSIAMAuthenticatorPath,
			ClusterName:              ts.cfg.Name,
		}); err != nil {
			return err
		}
		ts.lg.Info("writing KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
		if err := ioutil.WriteFile(ts.cfg.KubeConfigPath, buf.Bytes(), 0777); err != nil {
			return err
		}
		ts.lg.Info("wrote KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
		return ts.cfg.Sync()
	}

	args := []string{
		"eks",
		fmt.Sprintf("--region=%s", ts.cfg.Region),
		"update-kubeconfig",
		fmt.Sprintf("--name=%s", ts.cfg.Name),
		fmt.Sprintf("--kubeconfig=%s", ts.cfg.KubeConfigPath),
		"--verbose",
	}
	if ts.cfg.Parameters.ClusterResolverURL != "" {
		args = append(args, fmt.Sprintf("--endpoint=%s", ts.cfg.Parameters.ClusterResolverURL))
	}
	ts.lg.Info("writing KUBECONFIG with 'aws eks update-kubeconfig'",
		zap.String("kubeconfig-path", ts.cfg.KubeConfigPath),
		zap.String("aws-cli-path", ts.cfg.AWSCLIPath),
		zap.Strings("aws-args", args),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	ao, err := exec.New().CommandContext(
		ctx,
		ts.cfg.AWSCLIPath,
		args...,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'aws eks update-kubeconfig' failed (output %q, error %v)", string(ao), err)
	}
	ts.lg.Info("'aws eks update-kubeconfig' success", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
	return ts.cfg.Sync()
}

type kubeconfig struct {
	ClusterAPIServerEndpoint string
	ClusterCA                string
	AWSIAMAuthenticatorPath  string
	ClusterName              string
}

const tmplKUBECONFIG = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: {{ .ClusterAPIServerEndpoint }}
    certificate-authority-data: {{ .ClusterCA }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
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
      - {{ .ClusterName }}
`

func (ts *Tester) pollClusterInfo(timeout, interval time.Duration) error {
	retryStart := time.Now().UTC()
	ticker := time.NewTicker(interval)
	for time.Now().UTC().Sub(retryStart) < timeout {
		select {
		case <-ticker.C:
		case <-ts.stopCreationCh:
			return nil
		}
		d, err := ts.getClusterInfo()
		if err == nil {
			println()
			fmt.Println("kubectl cluster-info:")
			fmt.Println(d)
			println()
			return nil
		}
		ts.lg.Warn("failed to get cluster info; retrying", zap.Error(err))
	}
	return nil
}

func (ts *Tester) getClusterInfo() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	clusterInfoOut, err := exec.New().CommandContext(
		ctx,
		ts.cfg.KubectlPath,
		"--kubeconfig="+ts.cfg.KubeConfigPath,
		"cluster-info",
	).CombinedOutput()
	cancel()
	out := string(clusterInfoOut)
	if err != nil {
		return "", fmt.Errorf("'kubectl cluster-info' failed (output %q, error %v)", out, err)
	}
	if !strings.Contains(out, "is running at") {
		return "", fmt.Errorf("'kubectl cluster-info' not ready (output %q)", out)
	}
	return out, nil
}
