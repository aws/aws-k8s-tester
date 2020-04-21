package eks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

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

// https://docs.aws.amazon.com/cli/latest/reference/eks/update-kubeconfig.html
// https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html
// aws eks update-kubeconfig --name --role-arn --kubeconfig
func (ts *Tester) updateKUBECONFIG() (err error) {
	if ts.cfg.AWSIAMAuthenticatorPath != "" && ts.cfg.AWSIAMAuthenticatorDownloadURL != "" {
		tpl := template.Must(template.New("tmplKUBECONFIG").Parse(tmplKUBECONFIG))
		buf := bytes.NewBuffer(nil)
		if err = tpl.Execute(buf, kubeconfig{
			ClusterAPIServerEndpoint: ts.cfg.Status.ClusterAPIServerEndpoint,
			ClusterCA:                ts.cfg.Status.ClusterCA,
			AWSIAMAuthenticatorPath:  ts.cfg.AWSIAMAuthenticatorPath,
			ClusterName:              ts.cfg.Name,
		}); err != nil {
			return err
		}
		ts.lg.Info("writing KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
		if err = ioutil.WriteFile(ts.cfg.KubeConfigPath, buf.Bytes(), 0777); err != nil {
			return err
		}
		ts.lg.Info("wrote KUBECONFIG with aws-iam-authenticator", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
		return ts.cfg.Sync()
	}

	args := []string{
		ts.cfg.AWSCLIPath,
		"eks",
		fmt.Sprintf("--region=%s", ts.cfg.Region),
		"update-kubeconfig",
		fmt.Sprintf("--name=%s", ts.cfg.Name),
		fmt.Sprintf("--kubeconfig=%s", ts.cfg.KubeConfigPath),
		"--verbose",
	}
	if ts.cfg.Parameters.ResolverURL != "" {
		args = append(args, fmt.Sprintf("--endpoint=%s", ts.cfg.Parameters.ResolverURL))
	}
	ts.lg.Info("writing KUBECONFIG with 'aws eks update-kubeconfig'",
		zap.String("kubeconfig-path", ts.cfg.KubeConfigPath),
		zap.Strings("aws-args", args),
	)

	retryStart, waitDur := time.Now(), 3*time.Minute
	var output []byte
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.stopCreationCh:
			return errors.New("update-kubeconfig aborted")
		case <-ts.interruptSig:
			return errors.New("update-kubeconfig aborted")
		case <-time.After(5 * time.Second):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.lg.Warn("'aws eks update-kubeconfig' failed", zap.Error(err))
			if !strings.Contains(string(output), "Cluster status not active") || !strings.Contains(err.Error(), "exit") {
				return fmt.Errorf("'aws eks update-kubeconfig' failed (output %q, error %v)", string(output), err)
			}
			continue
		}
		ts.lg.Info("'aws eks update-kubeconfig' success", zap.String("kubeconfig-path", ts.cfg.KubeConfigPath))
		break
	}

	if err != nil {
		ts.lg.Warn("failed 'aws eks update-kubeconfig'", zap.String("output", string(output)), zap.Error(err))
		return err
	}

	ts.lg.Info("ran 'aws eks update-kubeconfig'", zap.String("output", string(output)))
	return nil
}
