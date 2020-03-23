package ng

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("writing ConfigMap", zap.String("instance-role-arn", ts.cfg.EKSConfig.AddOnNodeGroups.RoleARN))
	body, p, err := writeConfigMapAuth(ts.cfg.EKSConfig.AddOnNodeGroups.RoleARN)
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("applying ConfigMap")
	fmt.Printf("\naws-auth ConfigMap:\n%s\n\n", body)

	var output []byte
	// might take several minutes for DNS to propagate
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create ConfigMap aborted")
		case <-ts.cfg.Sig:
			return errors.New("create ConfigMap aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"apply", "--filename="+p,
		).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Printf("\n\"kubectl apply\" output:\n%s\n", out)
		if err == nil {
			break
		}
		// "configmap/aws-auth created" or "configmap/aws-auth unchanged"
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create ConfigMap failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("create ConfigMap failed (%v)", err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created ConfigMap")
	return ts.cfg.EKSConfig.Sync()
}

// TODO: use client-go
// https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html
const configMapAuthTempl = `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: aws-auth
  namespace: kube-system
data:
  mapRoles: |
    - rolearn: {{.NGInstanceRoleARN}}
      %s
      groups:
      - system:bootstrappers
      - system:nodes
`

type configMapAuth struct {
	NGInstanceRoleARN string
}

func writeConfigMapAuth(instanceRoleARN string) (body string, fpath string, err error) {
	kc := configMapAuth{NGInstanceRoleARN: instanceRoleARN}
	tpl := template.Must(template.New("configMapAuthTempl").Parse(configMapAuthTempl))
	buf := bytes.NewBuffer(nil)
	if err = tpl.Execute(buf, kc); err != nil {
		return "", "", err
	}
	// avoid '{{' conflicts with Go
	body = fmt.Sprintf(buf.String(), `username: system:node:{{EC2PrivateDNSName}}`)
	fpath, err = fileutil.WriteTempFile([]byte(body))
	return body, fpath, err
}
