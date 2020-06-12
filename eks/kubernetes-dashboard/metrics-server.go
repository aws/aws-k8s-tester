package kubernetesdashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// ref. https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html
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
func (ts *tester) installMetricsServer() error {
	ts.cfg.Logger.Info("writing metrics-server YAML")
	fpath, err := fileutil.WriteTempFile([]byte(metricsServerYAML))
	if err != nil {
		ts.cfg.Logger.Warn("failed to write metrics-server YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying metrics-server YAML", zap.String("path", fpath))

	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
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
		fmt.Printf("\n\"%s\" output:\n%s\n", applyCmd, out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create metrics-server failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("create metrics-server failed (%v)", err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}

	ts.cfg.Logger.Info("created metrics-server")

	return ts.waitDeploymentMetricsServer()
}

func (ts *tester) waitDeploymentMetricsServer() error {
	ts.cfg.Logger.Info("waiting for metrics-server Deployment")
	descArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"describe",
		"deployment",
		"metrics-server",
	}
	descCmd := strings.Join(descArgs, " ")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"%s\" output:\n%s\n\n", descCmd, out)

	ready := false
	waitDur := 3 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments("kube-system").
			Get(ctx, "metrics-server", metav1.GetOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get deployment", zap.Error(err))
			continue
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		if available && dresp.Status.AvailableReplicas >= 1 {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("Deployment not ready")
	}

	topNodeArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"top",
		"node",
	}
	topNodeCmd := strings.Join(topNodeArgs, " ")

	topNodeReady := false
	waitDur, retryStart = 7*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(5 * time.Second):
		}

		ts.cfg.Logger.Info("running kubectl top node")
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, topNodeArgs[0], topNodeArgs[1:]...).CombinedOutput()
		out := string(output)
		fmt.Printf("\n\n\"%s\" output:\n%s\n\n", topNodeCmd, out)
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

	ts.cfg.Logger.Info("waited for metrics-server Deployment")
	return ts.cfg.EKSConfig.Sync()
}
