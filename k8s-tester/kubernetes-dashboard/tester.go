// Package kubernetes_dashboard installs a Kubernetes dashboard.
// Replace https://github.com/aws/aws-k8s-tester/tree/v1.5.9/eks/kubernetes-dashboard.
package kubernetes_dashboard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	os_exec "os/exec"
	"path"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-k8s-tester/utils/http"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	k8s_client "k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

type Config struct {
	EnablePrompt bool

	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}

	ClientConfig *client.Config

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum-nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
}

const DefaultMinimumNodes int = 1

func New(cfg Config) k8s_tester.Tester {
	ccfg, err := client.CreateConfig(cfg.ClientConfig)
	if err != nil {
		cfg.Logger.Panic("failed to create client config", zap.Error(err))
	}
	cli, err := k8s_client.NewForConfig(ccfg)
	if err != nil {
		cfg.Logger.Panic("failed to create client", zap.Error(err))
	}

	return &tester{
		cfg: cfg,
		cli: cli,
	}
}

type tester struct {
	cfg Config
	cli k8s_client.Interface
}

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}

	if nodes, err := client.ListNodes(ts.cli); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}

	if err := ts.applyDashboardYAML(); err != nil {
		return err
	}

	if err := ts.checkDeploymentDashboard(); err != nil {
		return err
	}

	if err := ts.applyEKSAdminYAML(); err != nil {
		return err
	}

	token, err := ts.fetchAuthenticationToken()
	if err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\nKubernetes Dashboard Token:\n%s\n\n\n", token)

	if err := ts.checkKubeProxy(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	// TODO

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.EnablePrompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
// ref. https://github.com/kubernetes/dashboard/blob/master/aio/deploy/recommended.yaml
// ref. kubernetesui/dashboard
const dashboardYAML = `
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

apiVersion: v1
kind: Namespace
metadata:
  name: kubernetes-dashboard

---

apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard

---

kind: Service
apiVersion: v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  ports:
    - port: 443
      targetPort: 8443
  selector:
    k8s-app: kubernetes-dashboard

---

apiVersion: v1
kind: Secret
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-certs
  namespace: kubernetes-dashboard
type: Opaque

---

apiVersion: v1
kind: Secret
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-csrf
  namespace: kubernetes-dashboard
type: Opaque
data:
  csrf: ""

---

apiVersion: v1
kind: Secret
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-key-holder
  namespace: kubernetes-dashboard
type: Opaque

---

kind: ConfigMap
apiVersion: v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard-settings
  namespace: kubernetes-dashboard

---

kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
rules:
  # Allow Dashboard to get, update and delete Dashboard exclusive secrets.
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs", "kubernetes-dashboard-csrf"]
    verbs: ["get", "update", "delete"]
    # Allow Dashboard to get and update 'kubernetes-dashboard-settings' config map.
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["kubernetes-dashboard-settings"]
    verbs: ["get", "update"]
    # Allow Dashboard to get metrics.
  - apiGroups: [""]
    resources: ["services"]
    resourceNames: ["heapster", "dashboard-metrics-scraper"]
    verbs: ["proxy"]
  - apiGroups: [""]
    resources: ["services/proxy"]
    resourceNames: ["heapster", "http:heapster:", "https:heapster:", "dashboard-metrics-scraper", "http:dashboard-metrics-scraper"]
    verbs: ["get"]

---

kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
rules:
  # Allow Metrics Scraper to get metrics from the Metrics server
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list", "watch"]

---

apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kubernetes-dashboard
subjects:
  - kind: ServiceAccount
    name: kubernetes-dashboard
    namespace: kubernetes-dashboard

---

apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubernetes-dashboard
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubernetes-dashboard
subjects:
  - kind: ServiceAccount
    name: kubernetes-dashboard
    namespace: kubernetes-dashboard

---

kind: Deployment
apiVersion: apps/v1
metadata:
  labels:
    k8s-app: kubernetes-dashboard
  name: kubernetes-dashboard
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      k8s-app: kubernetes-dashboard
  template:
    metadata:
      labels:
        k8s-app: kubernetes-dashboard
    spec:
      containers:
        - name: kubernetes-dashboard
          image: kubernetesui/dashboard:v2.2.0
          imagePullPolicy: Always
          ports:
            - containerPort: 8443
              protocol: TCP
          args:
            - --auto-generate-certificates
            - --namespace=kubernetes-dashboard
            # Uncomment the following line to manually specify Kubernetes API server Host
            # If not specified, Dashboard will attempt to auto discover the API server and connect
            # to it. Uncomment only if the default does not work.
            # - --apiserver-host=http://my-address:port
          volumeMounts:
            - name: kubernetes-dashboard-certs
              mountPath: /certs
              # Create on-disk volume to store exec logs
            - mountPath: /tmp
              name: tmp-volume
          livenessProbe:
            httpGet:
              scheme: HTTPS
              path: /
              port: 8443
            initialDelaySeconds: 30
            timeoutSeconds: 30
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            runAsUser: 1001
            runAsGroup: 2001
      volumes:
        - name: kubernetes-dashboard-certs
          secret:
            secretName: kubernetes-dashboard-certs
        - name: tmp-volume
          emptyDir: {}
      serviceAccountName: kubernetes-dashboard
      nodeSelector:
        "kubernetes.io/os": linux
      # Comment the following tolerations if Dashboard must not be deployed on master
      tolerations:
        - key: node-role.kubernetes.io/master
          effect: NoSchedule

---

kind: Service
apiVersion: v1
metadata:
  labels:
    k8s-app: dashboard-metrics-scraper
  name: dashboard-metrics-scraper
  namespace: kubernetes-dashboard
spec:
  ports:
    - port: 8000
      targetPort: 8000
  selector:
    k8s-app: dashboard-metrics-scraper

---

kind: Deployment
apiVersion: apps/v1
metadata:
  labels:
    k8s-app: dashboard-metrics-scraper
  name: dashboard-metrics-scraper
  namespace: kubernetes-dashboard
spec:
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      k8s-app: dashboard-metrics-scraper
  template:
    metadata:
      labels:
        k8s-app: dashboard-metrics-scraper
      annotations:
        seccomp.security.alpha.kubernetes.io/pod: 'runtime/default'
    spec:
      containers:
        - name: dashboard-metrics-scraper
          image: kubernetesui/metrics-scraper:v1.0.6
          ports:
            - containerPort: 8000
              protocol: TCP
          livenessProbe:
            httpGet:
              scheme: HTTP
              path: /
              port: 8000
            initialDelaySeconds: 30
            timeoutSeconds: 30
          volumeMounts:
          - mountPath: /tmp
            name: tmp-volume
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            runAsUser: 1001
            runAsGroup: 2001
      serviceAccountName: kubernetes-dashboard
      nodeSelector:
        "kubernetes.io/os": linux
      # Comment the following tolerations if Dashboard must not be deployed on master
      tolerations:
        - key: node-role.kubernetes.io/master
          effect: NoSchedule
      volumes:
        - name: tmp-volume
          emptyDir: {}

`

func (ts *tester) applyDashboardYAML() error {
	ts.cfg.Logger.Info("writing dashboard YAML")
	fpath, err := file.WriteTempFile([]byte(dashboardYAML))
	if err != nil {
		ts.cfg.Logger.Warn("failed to write dashboard YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying dashboard YAML", zap.String("path", fpath))

	var output []byte
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create dashboard aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.ClientConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.ClientConfig.KubeConfigPath,
			"apply", "--filename="+fpath,
		).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\"kubectl apply\" dashboard output:\n%s\n", out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create dashboard failed", zap.Error(err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created dashboard")
	return nil
}

func (ts *tester) checkDeploymentDashboard() (err error) {
	timeout := 7 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cli,
		time.Minute,
		20*time.Second,
		"kubernetes-dashboard",
		"kubernetes-dashboard",
		1,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.ClientConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
				"--namespace=kubernetes-dashboard",
				"describe",
				"deployment",
				"kubernetes-dashboard",
			}
			descCmd := strings.Join(descArgs, " ")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
			cancel()
			if err != nil {
				ts.cfg.Logger.Warn("'kubectl describe deployment' failed", zap.Error(err))
			}
			out := string(output)
			fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", descCmd, out)
		}),
	)
	cancel()
	return err
}

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

func (ts *tester) applyEKSAdminYAML() error {
	ts.cfg.Logger.Info("writing eks-admin YAML")
	fpath, err := file.WriteTempFile([]byte(eksAdminYAML))
	if err != nil {
		ts.cfg.Logger.Warn("failed to write eks-admin YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying eks-admin YAML", zap.String("path", fpath))

	var output []byte
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create eks-admin aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(
			ctx,
			ts.cfg.ClientConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.ClientConfig.KubeConfigPath,
			"apply", "--filename="+fpath,
		).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\"kubectl apply\" eks-admin output:\n%s\n", out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create eks-admin failed", zap.Error(err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created eks-admin")
	return nil
}

func (ts *tester) fetchAuthenticationToken() (token string, err error) {
	ts.cfg.Logger.Info("fetching authentication token")

	var tb []byte
	waitDur := time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return "", errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ls, err := client.ListSecrets(ts.cfg.Logger, ts.cli, "kube-system", 10, 5*time.Second)
		if err != nil {
			return "", fmt.Errorf("failed to list secrets (%v)", err)
		}
		for _, v := range ls {
			if !strings.HasPrefix(v.Name, "eks-admin") {
				continue
			}
			tb = v.Data["token"]
			break
		}
		if len(tb) > 0 {
			break
		}
	}
	if len(tb) == 0 {
		return "", errors.New("authentication token not found")
	}
	ts.cfg.Logger.Info("fetched authentication token")

	token = string(tb)
	return token, nil
}

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
const defaultKubernetesDashboardURL = "http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/#/login"

func (ts *tester) checkKubeProxy() error {
	proxyArgs := []string{
		ts.cfg.ClientConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
		"proxy",
	}
	proxyCmd := strings.Join(proxyArgs, " ")

	ts.cfg.Logger.Info("starting Kubernetes Dashboard proxy", zap.String("cmd-path", ts.cfg.ClientConfig.KubectlPath))
	ctx, proxyCancel := context.WithCancel(context.Background())
	proxyCmdExec := os_exec.CommandContext(ctx, proxyArgs[0], proxyArgs[1:]...)
	proxyCmdExec.Stderr = os.Stderr
	proxyCmdExec.Stdout = os.Stdout
	if err := proxyCmdExec.Start(); err != nil {
		ts.cfg.Logger.Warn("failed to start kubectl proxy command", zap.Error(err))
		proxyCancel()
		if proxyCmdExec.Process != nil {
			proxyCmdExec.Process.Kill()
		}
		return err
	}
	ts.cfg.Logger.Info("started Kubernetes Dashboard proxy", zap.Int("pid", proxyCmdExec.Process.Pid))

	waitDur := time.Minute
	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("Kubernetes Dashboard proxy creation aborted")
		case <-time.After(5 * time.Second):
		}

		out, err := http.ReadInsecure(ts.cfg.Logger, ioutil.Discard, defaultKubernetesDashboardURL)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read Kubernetes Dashboard proxy; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		httpOutput := string(out)
		fmt.Fprintf(ts.cfg.LogWriter, "\nKubernetes Dashboard proxy output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `The Kubernetes Authors`) {
			ts.cfg.Logger.Info("read Kubernetes Dashboard proxy; exiting")
			break
		}

		ts.cfg.Logger.Warn("unexpected Kubernetes Dashboard proxy output; retrying")
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\nkubectl proxy command:\n%s\n", proxyCmd)
	fmt.Fprintf(ts.cfg.LogWriter, "\nKubernetes Dashboard URL:\n%s\n\n", defaultKubernetesDashboardURL)

	ts.cfg.Logger.Info("stopping Kubernetes Dashboard proxy")
	proxyCancel()
	if proxyCmdExec != nil && proxyCmdExec.Process != nil {
		err := proxyCmdExec.Process.Kill()
		if err != nil {
			ts.cfg.Logger.Warn("proxyCmdExec.Process.Kill failed", zap.Error(err))
		} else {
			ts.cfg.Logger.Info("ran proxyCmdExec.Process.Kill")
		}
	}
	if proxyCmdExec != nil && proxyCmdExec.Process != nil && proxyCmdExec.Process.Pid != 0 {
		err := syscall.Kill(-proxyCmdExec.Process.Pid, syscall.SIGKILL)
		if err != nil {
			ts.cfg.Logger.Warn("syscall.Kill failed", zap.Error(err))
		} else {
			ts.cfg.Logger.Info("ran syscall.Kill")
		}
	}
	ts.cfg.Logger.Info("stopped Kubernetes Dashboard proxy")

	return nil
}
