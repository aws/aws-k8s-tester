package kubernetesdashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
const eksAdminYAML = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: eks-admin
  namespace: kube-system

---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: eks-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: eks-admin
  namespace: kube-system

`

func (ts *tester) installEKSAdmin() error {
	ts.cfg.Logger.Info("writing eks-admin YAML")
	fpath, err := fileutil.WriteTempFile([]byte(eksAdminYAML))
	if err != nil {
		ts.cfg.Logger.Warn("failed to write eks-admin YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying eks-admin YAML", zap.String("path", fpath))

	var output []byte
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create eks-admin aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"apply", "--filename="+fpath,
		).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Printf("\n\"kubectl apply\" eks-admin output:\n%s\n", out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create eks-admin failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("create eks-admin failed (%v)", err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created eks-admin")
	return ts.fetchAuthenticationToken()
}

func (ts *tester) fetchAuthenticationToken() error {
	ts.cfg.Logger.Info("fetching authentication token")

	var token []byte
	waitDur := time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ls, err := ts.cfg.K8SClient.ListSecrets("kube-system", 10, 5*time.Second)
		if err != nil {
			return fmt.Errorf("failed to list secrets (%v)", err)
		}
		for _, v := range ls {
			if !strings.HasPrefix(v.Name, "eks-admin") {
				continue
			}
			token = v.Data["token"]
			break
		}
		if len(token) > 0 {
			break
		}
	}
	if len(token) == 0 {
		return errors.New("authentication token not found")
	}
	ts.cfg.Logger.Info("fetched authentication token")

	ts.cfg.EKSConfig.AddOnKubernetesDashboard.AuthenticationToken = string(token)
	fmt.Printf("\n\n\nKubernetes Dashboard Token:\n%s\n\n\n", ts.cfg.EKSConfig.AddOnKubernetesDashboard.AuthenticationToken)

	return ts.cfg.EKSConfig.Sync()
}
