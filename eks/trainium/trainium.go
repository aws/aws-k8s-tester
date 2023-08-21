package trainium

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

// Config defines Trainium configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Trainium tester.
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	InstallNeuronDriver() error
	CreateTrainiumJob() error
}

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

// New creates a new Job tester.
func New(cfg Config) Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

// neuron device plugin for Kubernetes from
// https://github.com/aws/aws-neuron-sdk/blob/master/docs/neuron-container-tools/k8s-neuron-device-plugin.yml
const neuronDriverTemplate = `
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: neuron-device-plugin
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - update
  - patch
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
  - update
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: neuron-device-plugin
  namespace: kube-system
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: neuron-device-plugin
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: neuron-device-plugin
subjects:
- kind: ServiceAccount
  name: neuron-device-plugin
  namespace: kube-system
---
# https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: neuron-device-plugin-daemonset
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: neuron-device-plugin-ds
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        name: neuron-device-plugin-ds
    spec:
      serviceAccount: neuron-device-plugin
      tolerations:
        - key: CriticalAddonsOnly
          operator: Exists
        - key: aws.amazon.com/neuron
          operator: Exists
          effect: NoSchedule
      # Mark this pod as a critical add-on; when enabled, the critical add-on
      # scheduler reserves resources for critical add-on pods so that they can
      # be rescheduled after a failure.
      # See https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/
      priorityClassName: "system-node-critical"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: "beta.kubernetes.io/instance-type"
                    operator: In
                    values:
                      - inf1.xlarge
                      - inf1.2xlarge
                      - inf1.6xlarge
                      - inf1.24xlarge
                      - trn1.2xlarge
                      - trn1.32xlarge
              - matchExpressions:
                  - key: "node.kubernetes.io/instance-type"
                    operator: In
                    values:
                      - inf1.xlarge
                      - inf1.2xlarge
                      - inf1.6xlarge
                      - inf1.24xlarge
                      - trn1.2xlarge
                      - trn1.32xlarge
      containers:
        - image: public.ecr.aws/neuron/neuron-device-plugin:1.9.3.0
          imagePullPolicy: IfNotPresent
          name: k8s-neuron-device-plugin-ctr
          env:
          - name: KUBECONFIG
            value: /etc/kubernetes/kubelet.conf
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
          volumeMounts:
            - name: device-plugin
              mountPath: /var/lib/kubelet/device-plugins
      volumes:
        - name: device-plugin
          hostPath:
            path: /var/lib/kubelet/device-plugins

`

func (ts *tester) InstallNeuronDriver() (err error) {
	ts.cfg.Logger.Info("starting tester.InstallNeuronDriver", zap.String("tester", pkgName))
	fpath, err := fileutil.WriteTempFile([]byte(neuronDriverTemplate))
	if err != nil {
		return err
	}
	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"apply",
		"-f",
		fpath,
	}
	applyCmd := strings.Join(applyArgs, " ")

	applied := false
	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("install neuron driver stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to install Neuron driver", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("installed neuron driver")
		break
	}
	if !applied {
		return errors.New("failed to install neuron driver")
	}

	ts.cfg.Logger.Info("checking neuron driver")

	descArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"describe",
		"daemonset.apps/neuron-device-plugin-daemonset",
	}
	descCmd := strings.Join(descArgs, " ")

	installed := false
	for time.Since(retryStart) < waitDur {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe daemonset.apps/neuron-device-plugin-daemonset", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmd, output)

		if strings.Contains(output, "SuccessfulCreate") {
			installed = true
			break
		}
	}

	if installed {
		ts.cfg.Logger.Info("checked neuron driver")
		return ts.cfg.EKSConfig.Sync()
	}
	ts.cfg.Logger.Warn("failed to install neuron driver")
	return errors.New("neuron driver installation failed")
}

const trainiumJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  labels:
    k8s-app: neuron-trainium-test
  name: trainium-client
spec:
  template:
    metadata:
      name: neuron-trainium-test
    spec:
      containers:
      - name: neuron-trainium-container
        image: {{ .Account }}.dkr.ecr.{{ .Region }}.amazonaws.com/neuron-trainium-test:1.0
        imagePullPolicy: IfNotPresent
        resources:
          limits:
            aws.amazon.com/neuron: 1
          requests:
            aws.amazon.com/neuron: 1
      # Do not restart containers after they exit
      restartPolicy: Never
  # of retries before marking as failed.
  backoffLimit: 3
`

func (ts *tester) CreateTrainiumJob() error {
	ts.cfg.Logger.Info("starting tester.CreateTrainiumJob", zap.String("tester", pkgName))
	tpl := template.Must(template.New("tmplTrainiumJobTemplate").Parse(trainiumJobTemplate))
	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, struct {
		Account string
		Region  string
	}{
		Account: ts.cfg.EKSConfig.Status.AWSAccountID,
		Region:  ts.cfg.EKSConfig.Region,
	}); err != nil {
		return err
	}
	fpath, err := fileutil.WriteTempFile(buf.Bytes())
	if err != nil {
		return err
	}
	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"apply",
		"-f",
		fpath,
	}
	applyCmd := strings.Join(applyArgs, " ")

	applied := false
	retryStart, waitDur := time.Now(), 10*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("create trainium job stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create trainium job", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("created trainium job")
		break
	}
	if !applied {
		return errors.New("failed to create trainium job")
	}

	ts.cfg.Logger.Info("checking trainium job")

	getArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"get",
		"pods",
		"--selector=job-name=trainium-client",
	}
	getCmd := strings.Join(getArgs, " ")

	completed := false
	for time.Since(retryStart) < waitDur {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, err := exec.New().CommandContext(ctx, getArgs[0], getArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl get pods --selector=job-name=trainium-client", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getCmd, output)

		if strings.Contains(output, "Completed") {
			completed = true
			break
		}
	}

	if completed {
		ts.cfg.Logger.Info("checked trainium job")
		return ts.cfg.EKSConfig.Sync()
	}
	ts.cfg.Logger.Warn("failed to test trainium job")
	return errors.New("trainium job failed")
}
