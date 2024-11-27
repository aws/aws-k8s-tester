package neuron

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

// Config defines Neuron configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Neuron tester.
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	InstallNeuronDriver() error
	InstallBertService() error
	CreateBertJob() error
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
                      - trn2.48xlarge
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
                      - trn2.48xlarge
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

const bertServiceTemplate = `
---
kind: Service
apiVersion: v1
metadata:
  name: eks-neuron-test-bert-service
  labels:
    app: eks-neuron-test-bert-service
spec:
  type: ClusterIP
  ports:
    - name: grpc-tf-serving
      port: 8500
      targetPort: 8500
    - name: http-tf-serving
      port: 8501
      targetPort: 8501
  selector:
    app: eks-neuron-test-bert-service
    role: master
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: eks-neuron-test-bert-service
  labels:
    app: eks-neuron-test-bert-service
    role: master
spec:
  replicas: 1
  selector:
    matchLabels:
      app: eks-neuron-test-bert-service
      role: master
  template:
    metadata:
      labels:
        app: eks-neuron-test-bert-service
        role: master
    spec:
      initContainers:
        - name: eks-neuron-bert-model
          image: {{ .Account }}.dkr.ecr.{{ .Region }}.amazonaws.com/tensorflow-neuron-bert-model:1.0
          volumeMounts:
            - name: models
              mountPath: /models
          command: ["mv", "/model", "/models/"]
      containers:
        - name: eks-neuron-test-bert-service
          image: {{ .Account }}.dkr.ecr.{{ .Region }}.amazonaws.com/neuron-test-images/tensorflow-inference-neuron:1.15.5-neuron-py37-sdk1.17.1-ubuntu18.04-v1.1
          ports:
            - containerPort: 8500
            - containerPort: 8501
          imagePullPolicy: IfNotPresent
          env:
            - name: AWS_REGION
              value: {{ .Region }}
            - name: S3_USE_HTTPS
              value: "1"
            - name: S3_VERIFY_SSL
              value: "0"
            - name: S3_ENDPOINT
              value: s3.{{ .Region }}.amazonaws.com
            - name: AWS_LOG_LEVEL
              value: "3"
          resources:
            limits:
              cpu: 4
              memory: 4Gi
              aws.amazon.com/neuron: 1
            requests:
              cpu: "1"
              memory: 1Gi
          volumeMounts:
          - name: models
            mountPath: /models
      volumes:
      - name: models
        emptyDir: {}
`

func (ts *tester) InstallBertService() error {
	ts.cfg.Logger.Info("starting tester.InstallBertService", zap.String("tester", pkgName))
	tpl := template.Must(template.New("tmplBertServiceTemplate").Parse(bertServiceTemplate))
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
	retryStart, waitDur := time.Now(), 20*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("install bert service stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to install bert service", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("installed bert service")
		break
	}
	if !applied {
		return errors.New("failed to install bert service")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	_, err = k8s_client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		20*time.Second,
		"default",
		"eks-neuron-test-bert-service",
		1,
		k8s_client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"describe",
				"deployment",
				"eks-neuron-test-bert-service",
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
	if err != nil {
		return errors.New("failed to deploy bert service")
	}
	return ts.cfg.EKSConfig.Sync()
}

const bertJobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  labels:
    k8s-app: bert-client-app
  # Unique key of the Job instance
  name: bert-client
spec:
  template:
    metadata:
      name: bert-client
    spec:
      containers:
      - name: bert-client-container
        image: {{ .Account }}.dkr.ecr.{{ .Region }}.amazonaws.com/bert-client:2.0
        imagePullPolicy: IfNotPresent
        command: ["/bin/sh","-c"]
        args:
         - python3 bert_client.py eks-neuron-test-bert-service:8500 model;

      # Do not restart containers after they exit
      restartPolicy: Never
  # of retries before marking as failed.
  backoffLimit: 3
`

func (ts *tester) CreateBertJob() error {
	ts.cfg.Logger.Info("starting tester.CreateBertJob", zap.String("tester", pkgName))
	tpl := template.Must(template.New("tmplBertJobTemplate").Parse(bertJobTemplate))
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
			ts.cfg.Logger.Warn("create bert job stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create bert job", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("created bert job")
		break
	}
	if !applied {
		return errors.New("failed to create bert job")
	}

	ts.cfg.Logger.Info("checking bert job")

	getArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"get",
		"pods",
		"--selector=job-name=bert-client",
	}
	getCmd := strings.Join(getArgs, " ")

	completed := false
	for time.Since(retryStart) < waitDur {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, err := exec.New().CommandContext(ctx, getArgs[0], getArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl get pods --selector=job-name=bert-client", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", getCmd, output)

		if strings.Contains(output, "Completed") {
			completed = true
			break
		}
	}

	if completed {
		ts.cfg.Logger.Info("checked bert job")
		return ts.cfg.EKSConfig.Sync()
	}
	ts.cfg.Logger.Warn("failed to test bert job")
	return errors.New("bert job failed")
}
