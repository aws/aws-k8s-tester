// Package metrics_server a simple metrics server.
// Replace https://github.com/aws/aws-k8s-tester/blob/v1.5.9/eks/metrics-server/metrics-server.go.
package metrics_server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/file"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if err := ts.applyMetricsServerYAML(); err != nil {
		return err
	}

	if err := ts.checkDeployment(); err != nil {
		return err
	}

	if err := ts.checkMetrics(); err != nil {
		return err
	}

	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}

	var errs []string

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, err.Error())
	}

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
// ref. https://github.com/kubernetes-sigs/metrics-server/releases
// ref. https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.3.6/components.yaml
const metricsServerYAML = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:aggregated-metrics-reader
  labels:
    rbac.authorization.k8s.io/aggregate-to-view: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system

---
apiVersion: apiregistration.k8s.io/v1beta1
kind: APIService
metadata:
  name: v1beta1.metrics.k8s.io
spec:
  service:
    name: metrics-server
    namespace: kube-system
  group: metrics.k8s.io
  version: v1beta1
  insecureSkipTLSVerify: true
  groupPriorityMinimum: 100
  versionPriority: 100

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-server
  namespace: kube-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  template:
    metadata:
      name: metrics-server
      labels:
        k8s-app: metrics-server
    spec:
      serviceAccountName: metrics-server
      volumes:
      # mount in tmp so we can safely use from-scratch images and/or read-only containers
      - name: tmp-dir
        emptyDir: {}
      containers:
      - name: metrics-server
        image: k8s.gcr.io/metrics-server-amd64:v0.3.6
        imagePullPolicy: IfNotPresent
        args:
        - --cert-dir=/tmp
        - --secure-port=4443
        - --kubelet-insecure-tls
        - --kubelet-preferred-address-types=InternalIP
        ports:
        - name: main-port
          containerPort: 4443
          protocol: TCP
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
        volumeMounts:
        - name: tmp-dir
          mountPath: /tmp
      nodeSelector:
        kubernetes.io/os: linux
        kubernetes.io/arch: "amd64"

---
apiVersion: v1
kind: Service
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    kubernetes.io/name: "Metrics-server"
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: metrics-server
  ports:
  - port: 443
    protocol: TCP
    targetPort: main-port

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  - nodes/stats
  - namespaces
  - configmaps
  verbs:
  - get
  - list
  - watch

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system


`

// ref. https://github.com/kubernetes-sigs/metrics-server
func (ts *tester) applyMetricsServerYAML() error {
	ts.cfg.Logger.Info("writing metrics-server YAML")
	fpath, err := file.WriteTempFile([]byte(metricsServerYAML))
	if err != nil {
		ts.cfg.Logger.Warn("failed to write metrics-server YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying metrics-server YAML", zap.String("path", fpath))

	applyArgs := []string{
		ts.cfg.ClientConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
		"apply",
		"--filename=" + fpath,
	}
	applyCmd := strings.Join(applyArgs, " ")

	var output []byte
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create metrics-server aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\"%s\" output:\n%s\n", applyCmd, out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create metrics-server failed", zap.Error(err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created metrics-server")
	return nil
}

const deploymentName = "metrics-server"

func (ts *tester) checkDeployment() (err error) {
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
		"kube-system",
		deploymentName,
		1,
		client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.ClientConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
				"--namespace=kube-system",
				"describe",
				"deployment",
				deploymentName,
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

func (ts *tester) checkMetrics() error {
	logArgs := []string{
		ts.cfg.ClientConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
		"--namespace=kube-system",
		"logs",
		"--selector=k8s-app=metrics-server",
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	topNodeArgs := []string{
		ts.cfg.ClientConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.ClientConfig.KubeConfigPath,
		"top",
		"node",
	}
	topNodeCmd := strings.Join(topNodeArgs, " ")

	topNodeReady := false
	waitDur, retryStart := 30*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to run kubectl logs", zap.Error(err))
			continue
		}
		out := string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", logsCmd, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(ctx, topNodeArgs[0], topNodeArgs[1:]...).CombinedOutput()
		out = string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"%s\" output:\n%s\n\n", topNodeCmd, out)
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to run kubectl top node", zap.Error(err))
			continue
		}
		if strings.Contains(out, "MEMORY") {
			topNodeReady = true
			break
		}
	}
	if !topNodeReady {
		return fmt.Errorf("%q not ready", topNodeCmd)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := meta_v1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cli.
		AppsV1().
		Deployments("kube-system").
		Delete(
			ctx,
			deploymentName,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return nil
}
