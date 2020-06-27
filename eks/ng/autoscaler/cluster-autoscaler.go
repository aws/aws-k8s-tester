package autoscaler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// auto discover
// ref. https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/cloudprovider/aws/examples/cluster-autoscaler-autodiscover.yaml
const clusterAutoscalerYAML = `
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
  name: cluster-autoscaler
  namespace: kube-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-autoscaler
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
rules:
  - apiGroups: [""]
    resources: ["events", "endpoints"]
    verbs: ["create", "patch"]
  - apiGroups: [""]
    resources: ["pods/eviction"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["update"]
  - apiGroups: [""]
    resources: ["endpoints"]
    resourceNames: ["cluster-autoscaler"]
    verbs: ["get", "update"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["watch", "list", "get", "update"]
  - apiGroups: [""]
    resources:
      - "pods"
      - "services"
      - "replicationcontrollers"
      - "persistentvolumeclaims"
      - "persistentvolumes"
    verbs: ["watch", "list", "get"]
  - apiGroups: ["extensions"]
    resources: ["replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["watch", "list"]
  - apiGroups: ["apps"]
    resources: ["statefulsets", "replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses", "csinodes"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["batch", "extensions"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "patch"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["create"]
  - apiGroups: ["coordination.k8s.io"]
    resourceNames: ["cluster-autoscaler"]
    resources: ["leases"]
    verbs: ["get", "update"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create","list","watch"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["cluster-autoscaler-status", "cluster-autoscaler-priority-expander"]
    verbs: ["delete", "get", "update", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-autoscaler
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-autoscaler
subjects:
  - kind: ServiceAccount
    name: cluster-autoscaler
    namespace: kube-system

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    k8s-addon: cluster-autoscaler.addons.k8s.io
    k8s-app: cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cluster-autoscaler
subjects:
  - kind: ServiceAccount
    name: cluster-autoscaler
    namespace: kube-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
  labels:
    app: cluster-autoscaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    metadata:
      labels:
        app: cluster-autoscaler
      annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: '8085'
        cluster-autoscaler.kubernetes.io/safe-to-evict: 'false'
    spec:
      serviceAccountName: cluster-autoscaler
      containers:
{{ if ne .ImageURI "" }}{{.ImageURI}}{{ end }}
          name: cluster-autoscaler
          resources:
            limits:
              cpu: 100m
              memory: 300Mi
            requests:
              cpu: 100m
              memory: 300Mi
          command:
            - ./cluster-autoscaler
            - --v=4
            - --stderrthreshold=info
            - --cloud-provider=aws
            - --skip-nodes-with-local-storage=false
            - --expander=least-waste
{{ if ne .NodeGroupAutoDiscovery "" }}{{.NodeGroupAutoDiscovery}}{{ end }}
            - --balance-similar-node-groups
            - --skip-nodes-with-system-pods=false
          volumeMounts:
            - name: ssl-certs
              mountPath: /etc/ssl/certs/ca-certificates.crt
              readOnly: true
          imagePullPolicy: "Always"
      volumes:
        - name: ssl-certs
          hostPath:
            path: "/etc/ssl/certs/ca-bundle.crt"
`

// ref. https://github.com/kubernetes/autoscaler/releases
var caImages = map[string]string{
	"1.16": `        - image: us.gcr.io/k8s-artifacts-prod/autoscaling/cluster-autoscaler:v1.16.5`,
	"1.17": `        - image: us.gcr.io/k8s-artifacts-prod/autoscaling/cluster-autoscaler:v1.17.2`,
}

const (
	nodeGroupAuotDiscoveryData      = `            - --node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,k8s.io/cluster-autoscaler/`
	clusterAutoscalerDeploymentName = "cluster-autoscaler"
)

type caSpecData struct {
	ImageURI               string
	NodeGroupAutoDiscovery string
}

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// ClusterAutoscaler defines cluster autoscaler operation.
type ClusterAutoscaler interface {
	Create() error
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new cluster autoscaler.
func New(cfg Config) ClusterAutoscaler {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	needInstall := false
	for _, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		if cur.ClusterAutoscaler != nil && cur.ClusterAutoscaler.Enable {
			needInstall = true
			break
		}
	}
	if !needInstall {
		ts.cfg.Logger.Info("no NG enables CA; skipping")
		return nil
	}
	return ts.installCA()
}

func (ts *tester) installCA() error {
	ts.cfg.Logger.Info("creating CA using kubectl", zap.String("name", ts.cfg.EKSConfig.Name))
	var ok bool
	var caData = caSpecData{}
	caData.ImageURI, ok = caImages[ts.cfg.EKSConfig.Parameters.Version]
	if !ok {
		return fmt.Errorf("no CA found for %q", ts.cfg.EKSConfig.Parameters.Version)
	}
	caData.NodeGroupAutoDiscovery = nodeGroupAuotDiscoveryData + ts.cfg.EKSConfig.Name
	tpl := template.Must(template.New("TemplateCA").Parse(clusterAutoscalerYAML))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, caData); err != nil {
		return err
	}
	ts.cfg.Logger.Info("writing cluster autoscaler YAML")
	fpath, err := fileutil.WriteTempFile(buf.Bytes())
	if err != nil {
		ts.cfg.Logger.Warn("failed to write cluster-autoscaler YAML", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("applying cluster-autoscaler YAML", zap.String("path", fpath))

	var output []byte
	waitDur := 5 * time.Minute
	retryStart := time.Now()
	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"apply",
		"-f",
		fpath,
	}
	applyCmd := strings.Join(applyArgs, " ")
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("create CA aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err == nil {
			break
		}
		if strings.Contains(out, " created") || strings.Contains(out, " unchanged") {
			err = nil
			break
		}

		ts.cfg.Logger.Warn("create CA failed", zap.Error(err))
		ts.cfg.EKSConfig.RecordStatus(fmt.Sprintf("create CA failed (%v)", err))
	}
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed %v (output %q)", err, string(output))
	}
	ts.cfg.Logger.Info("created cluster autoscaler")

	return ts.waitDeploymentCA()
}

func (ts *tester) waitDeploymentCA() error {
	ts.cfg.Logger.Info("waiting for cluster autoscaler Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"describe",
		"deployment",
		clusterAutoscalerDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

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
			Get(ctx, "cluster-autoscaler", metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
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

	ts.cfg.Logger.Info("waited for cluster autoscaler Deployment")
	return ts.cfg.EKSConfig.Sync()
}
