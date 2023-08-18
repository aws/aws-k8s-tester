// Package gpu implements GPU plugin.
package gpu

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

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-sdk-go/service/eks"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines GPU configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines GPU tester.
type Tester interface {
	// Name returns the name of the tester.
	Name() string
	DeployMPIOperator() error
	// InstallNvidiaDriver installs the Nvidia device plugin for Kubernetes.
	// After GPU worker nodes join the cluster, one must apply the Nvidia
	// device plugin for Kubernetes as a DaemonSet.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
	// ref. https://github.com/NVIDIA/k8s-device-plugin
	InstallNvidiaDriver() error
	// Horovod
	//# https://github.com/horovod/horovod
	// A sample horovod MPIJob that trains some model using an Nvidia GPU
	// MPI operator creates pods according to the MPIJob
	// https://github.com/horovod/horovod/blob/master/examples/tensorflow2/tensorflow2_mnist.py
	CreateMPIJob() error
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

func (ts *tester) DeployMPIOperator() error {
	ts.cfg.Logger.Info("starting tester.DeployMPIOperator", zap.String("tester", pkgName))
	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"apply",
		"-f",
		"https://raw.githubusercontent.com/kubeflow/mpi-operator/v0.4.0/deploy/v2beta1/mpi-operator.yaml",
	}
	applyCmd := strings.Join(applyArgs, " ")

	applied := false
	retryStart, waitDur := time.Now(), 5*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("deploy MPI operator stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to deploy MPI operator", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("deployed MPI operator")
		break
	}
	if !applied {
		return errors.New("failed to deploy MPI operator")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	_, err := k8s_client.WaitForDeploymentCompletes(
		ctx,
		ts.cfg.Logger,
		ts.cfg.LogWriter,
		ts.cfg.Stopc,
		ts.cfg.K8SClient,
		time.Minute,
		20*time.Second,
		"mpi-operator",
		"mpi-operator",
		1,
		k8s_client.WithQueryFunc(func() {
			descArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=mpi-operator",
				"describe",
				"deployment",
				"mpi-operator",
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
    return errors.New("failed to deploy MPI operator")
  }
	return ts.cfg.EKSConfig.Sync()
}

// https://github.com/NVIDIA/k8s-device-plugin/blob/main/nvidia-device-plugin.yml
const nvidiaDriverTemplate = `
# nvidia device plugin needed to allow test nodes to request a GPU resource
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nvidia-device-plugin-daemonset
  namespace: mpi-operator
spec:
  selector:
    matchLabels:
      name: nvidia-device-plugin-ds
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      # Mark this pod as a critical add-on; when enabled, the critical add-on scheduler
      # reserves resources for critical add-on pods so that they can be rescheduled after
      # a failure.  This annotation works in tandem with the toleration below.
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        name: nvidia-device-plugin-ds
    spec:
      tolerations:
      # Allow this pod to be rescheduled while the node is in "critical add-ons only" mode.
      # This, along with the annotation above marks this pod as a critical add-on.
      - key: CriticalAddonsOnly
        operator: Exists
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      containers:
      - image: nvcr.io/nvidia/k8s-device-plugin:v0.14.1
        name: nvidia-device-plugin-ctr
        args: ["--fail-on-init-error=false"]
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

// https://github.com/NVIDIA/k8s-device-plugin/releases
// https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
// https://github.com/NVIDIA/k8s-device-plugin
// https://github.com/NVIDIA/k8s-device-plugin/blob/master/nvidia-device-plugin.yml
// https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/1.0.0-beta5/nvidia-device-plugin.yml
func (ts *tester) InstallNvidiaDriver() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() && !ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ts.cfg.Logger.Info("skipping nvidia driver install")
		return nil
	}

	ts.cfg.Logger.Info("starting tester.InstallNvidiaDriver", zap.String("tester", pkgName))
	fpath, err := fileutil.WriteTempFile([]byte(nvidiaDriverTemplate))
	if err != nil {
		return err
	}
	applyArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=mpi-operator",
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
			ts.cfg.Logger.Warn("install nvidia GPU driver stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create nvidia GPU driver", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("created nvidia GPU driver")
		break
	}
	if !applied {
		return errors.New("failed to install nvidia GPU driver")
	}

	if ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		cnt := 0
		for _, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
			if cur.AMIType == ec2config.AMITypeAL2X8664GPU {
				cnt++
			}
		}

		waitDur := 5 * time.Minute
		var items []v1.Node
		retryStart := time.Now()

		readyNGs := make(map[string]struct{})
		for time.Since(retryStart) < waitDur {
			if len(readyNGs) == cnt {
				break
			}
			for ngName, cur := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
				if cur.AMIType != ec2config.AMITypeAL2X8664GPU {
					ts.cfg.Logger.Warn("skipping non-GPU AMI", zap.String("ng-name", ngName))
					continue
				}
				if _, ok := readyNGs[ngName]; ok {
					ts.cfg.Logger.Info("skipping already ready mng", zap.String("ng-name", ngName))
					continue
				}
				ts.cfg.Logger.Info("listing GPU nodes via client-go", zap.String("ng-name", ngName))
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(
					ctx,
					metav1.ListOptions{
						// TODO: filter by GPU?
						// FieldSelector: fields.OneTermEqualSelector("metadata.name", "GPU").String(),
					},
				)
				cancel()
				if err != nil {
					ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
					time.Sleep(5 * time.Second)
					continue
				}
				items = nodes.Items
				ts.cfg.Logger.Info("listed GPU nodes via client-go", zap.String("ng-name", ngName), zap.Int("nodes", len(items)))

				foundReady := int32(0)
				for _, node := range items {
					labels := node.GetLabels()
					if labels["NGName"] != ngName {
						continue
					}
					nodeName := node.GetName()

					ts.cfg.Logger.Info("checking node-info conditions",
						zap.String("node-name", nodeName),
						zap.String("labels", fmt.Sprintf("%+v", labels)),
						zap.String("allocatable", fmt.Sprintf("%+v", node.Status.Allocatable)),
					)
					for _, cond := range node.Status.Conditions {
						if cond.Type != v1.NodeReady {
							continue
						}
						ts.cfg.Logger.Info("node info",
							zap.String("node-name", nodeName),
							zap.String("type", fmt.Sprintf("%s", cond.Type)),
							zap.String("status", fmt.Sprintf("%s", cond.Status)),
						)
						if cond.Status == v1.ConditionTrue {
							foundReady++
						}
					}
				}
				ts.cfg.Logger.Info("nodes",
					zap.Int32("current-ready-nodes", foundReady),
					zap.Int32("min-ready-nodes", cur.ASGMinSize),
					zap.Int32("desired-ready-nodes", cur.ASGDesiredCapacity),
				)
				time.Sleep(5 * time.Second)

				if foundReady >= cur.ASGMinSize {
					readyNGs[ngName] = struct{}{}
					break
				}
			}
		}
	}

	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		cnt := 0
		for _, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			if cur.AMIType == eks.AMITypesAl2X8664Gpu {
				cnt++
			}
		}

		waitDur := 5 * time.Minute
		var items []v1.Node
		retryStart := time.Now()

		readyMNGs := make(map[string]struct{})
		for time.Since(retryStart) < waitDur {
			if len(readyMNGs) == cnt {
				break
			}
			for mngName, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
				if cur.AMIType != eks.AMITypesAl2X8664Gpu {
					ts.cfg.Logger.Warn("skipping non-GPU AMI", zap.String("mng-name", mngName))
					continue
				}
				if _, ok := readyMNGs[mngName]; ok {
					ts.cfg.Logger.Info("skipping already ready mng", zap.String("mng-name", mngName))
					continue
				}
				ts.cfg.Logger.Info("listing GPU nodes via client-go", zap.String("mng-name", mngName))
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(
					ctx,
					metav1.ListOptions{
						// TODO: filter by GPU?
						// FieldSelector: fields.OneTermEqualSelector("metadata.name", "GPU").String(),
					},
				)
				cancel()
				if err != nil {
					ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
					time.Sleep(5 * time.Second)
					continue
				}
				items = nodes.Items
				ts.cfg.Logger.Info("listed GPU nodes via client-go", zap.String("mng-name", mngName), zap.Int("nodes", len(items)))

				foundReady := 0
				for _, node := range items {
					labels := node.GetLabels()
					if labels["NGName"] != mngName {
						continue
					}
					nodeName := node.GetName()
					ts.cfg.Logger.Info("checking node-info conditions", zap.String("node-name", nodeName), zap.String("labels", fmt.Sprintf("%+v", labels)))
					for _, cond := range node.Status.Conditions {
						if cond.Type != v1.NodeReady {
							continue
						}
						ts.cfg.Logger.Info("node info",
							zap.String("node-name", nodeName),
							zap.String("type", fmt.Sprintf("%s", cond.Type)),
							zap.String("status", fmt.Sprintf("%s", cond.Status)),
						)
						if cond.Status == v1.ConditionTrue {
							foundReady++
						}
					}
				}
				ts.cfg.Logger.Info("nodes",
					zap.Int("current-ready-nodes", foundReady),
					zap.Int("min-ready-nodes", cur.ASGMinSize),
					zap.Int("desired-ready-nodes", cur.ASGDesiredCapacity),
				)
				time.Sleep(5 * time.Second)

				if foundReady >= cur.ASGMinSize {
					readyMNGs[mngName] = struct{}{}
					break
				}
			}
		}
	}

	ts.cfg.EKSConfig.Sync()
	return nil
}

const mpiJobTemplate = `
apiVersion: kubeflow.org/v2beta1
kind: MPIJob
metadata:
  name: gpu-test
  namespace: mpi-operator
spec:
  slotsPerWorker: 4
  runPolicy:
    cleanPodPolicy: Running
  mpiImplementation: OpenMPI
  mpiReplicaSpecs:
    Launcher:
      replicas: 1
      template:
         spec:
           restartPolicy: OnFailure
           containers:
           - image: {{ .Account }}.dkr.ecr.{{ .Region }}.amazonaws.com/pytorch-training:1.9.1-gpu-py38-cu111-ubuntu20.04-v1.7
             name: gpu-test
             command:
              - mpirun
              - --allow-run-as-root
              - -np
              - "1"
              - -mca
              - btl_tcp_if_exclude
              - lo
              - -mca
              - pml
              - ob1
              - -mca
              - btl
              - ^openib
              - --bind-to
              - none
              - -map-by
              - slot
              - -x
              - LD_LIBRARY_PATH
              - -x
              - PATH
              - -x
              - NCCL_SOCKET_IFNAME=eth0
              - -x
              - NCCL_DEBUG=INFO
              - -x
              - MXNET_CUDNN_AUTOTUNE_DEFAULT=0
              - python
              - -c
              - import os; os.system("git clone https://github.com/pytorch/examples.git /pytorch-examples"); os.system("git -C pytorch-examples checkout 0f0c9131ca5c79d1332dce1f4c06fe942fbdc665"); os.system("python /pytorch-examples/mnist/main.py --epochs 3")
             resources:
               limits:
                 nvidia.com/gpu: 1
`

func (ts *tester) CreateMPIJob() error {
	ts.cfg.Logger.Info("starting tester.CreateMPIJob", zap.String("tester", pkgName))
	tpl := template.Must(template.New("tmplMPIJobTemplate").Parse(mpiJobTemplate))
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
			ts.cfg.Logger.Warn("create MPI job stopped")
			return nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		output, err := exec.New().CommandContext(ctx, applyArgs[0], applyArgs[1:]...).CombinedOutput()
		cancel()
		out := strings.TrimSpace(string(output))
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", applyCmd, out)
		if err != nil {
			ts.cfg.Logger.Warn("failed to create MPI job", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		applied = true
		ts.cfg.Logger.Info("created MPI job")
		break
	}
	if !applied {
		return errors.New("failed to create MPI job")
	}

	ts.cfg.Logger.Info("checking MPI job")

	descArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=mpi-operator",
		"describe",
		"mpijob/gpu-test",
	}
	descCmd := strings.Join(descArgs, " ")

	installed := false
	for time.Since(retryStart) < waitDur {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		out, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe mpijob/gpu-test", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descCmd, output)

		if strings.Contains(output, "MPIJobSucceeded") {
			installed = true
			break
		}
	}

	if installed {
		ts.cfg.Logger.Info("checked MPI job")
		return ts.cfg.EKSConfig.Sync()
	}
	ts.cfg.Logger.Warn("failed to test MPI job")
	return errors.New("MPI job failed")
}
