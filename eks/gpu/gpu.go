// Package gpu implements GPU plugin.
package gpu

import (
	"context"
	"errors"
	"fmt"
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
	"k8s.io/apimachinery/pkg/api/resource"
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
	// InstallNvidiaDriver installs the Nvidia device plugin for Kubernetes.
	// After GPU worker nodes join the cluster, one must apply the Nvidia
	// device plugin for Kubernetes as a DaemonSet.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
	// ref. https://github.com/NVIDIA/k8s-device-plugin
	InstallNvidiaDriver() error
	// CreateNvidiaSMI launches a pod manifest that launches a Cuda container that
	// runs "nvidia-smi" on a GPU worker node.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
	CreateNvidiaSMI() error
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

// https://github.com/NVIDIA/k8s-device-plugin/blob/master/nvidia-device-plugin.yml
// https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/1.0.0-beta5/nvidia-device-plugin.yml
// kubectl apply -f apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/1.0.0-beta5/nvidia-device-plugin.yml
const nvidiaDriverTemplate = `
# Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
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

apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nvidia-device-plugin-daemonset
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: nvidia-device-plugin-ds
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      # This annotation is deprecated. Kept here for backward compatibility
      # See https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ""
      labels:
        name: nvidia-device-plugin-ds
    spec:
      tolerations:
      # This toleration is deprecated. Kept here for backward compatibility
      # See https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/
      - key: CriticalAddonsOnly
        operator: Exists
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
      # Mark this pod as a critical add-on; when enabled, the critical add-on
      # scheduler reserves resources for critical add-on pods so that they can
      # be rescheduled after a failure.
      # See https://kubernetes.io/docs/tasks/administer-cluster/guaranteed-scheduling-critical-addon-pods/
      priorityClassName: "system-node-critical"
      containers:
      - image: nvidia/k8s-device-plugin:1.0.0-beta5
        name: nvidia-device-plugin-ctr
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

				foundReady := int64(0)
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
					zap.Int64("current-ready-nodes", foundReady),
					zap.Int64("min-ready-nodes", cur.ASGMinSize),
					zap.Int64("desired-ready-nodes", cur.ASGDesiredCapacity),
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

/*
https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
takes about 1-min to finish

kubectl apply -f nvidia-smi.yaml
kubectl logs nvidia-smi

+-----------------------------------------------------------------------------+
| NVIDIA-SMI 418.87.00    Driver Version: 418.87.00    CUDA Version: 10.1     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|===============================+======================+======================|
|   0  Tesla V100-SXM2...  On   | 00000000:00:1D.0 Off |                    0 |
| N/A   43C    P0    41W / 300W |      0MiB / 16130MiB |      1%      Default |
+-------------------------------+----------------------+----------------------+

+-----------------------------------------------------------------------------+
| Processes:                                                       GPU Memory |
|  GPU       PID   Type   Process name                             Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
*/
func (ts *tester) CreateNvidiaSMI() error {
	ts.cfg.Logger.Info("starting tester.CreateNvidiaSMI", zap.String("tester", pkgName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.
		KubernetesClientSet().
		CoreV1().
		Pods("default").
		Create(
			ctx,
			&v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "nvidia-smi",
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name:  "nvidia-smi",
							Image: "nvidia/cuda:9.2-devel",
							Args:  []string{"nvidia-smi"},
							Resources: v1.ResourceRequirements{
								Limits: map[v1.ResourceName]resource.Quantity{
									v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
								},
							},
						},
					},

					// DO NOT SET node selector, it fails with
					// "Warning  FailedScheduling  20s (x2 over 91s)  default-scheduler  0/5 nodes are available: 3 node(s) didn't match node selector, 5 Insufficient nvidia.com/gpu."
					// NodeSelector: map[string]string{
					// 	"AMIType": ec2config.AMITypeAL2X8664GPU,
					// },
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("checking nvidia-smi")
	select {
	case <-ts.cfg.Stopc:
		return errors.New("nvidia-smi install aborted")
	case <-time.After(time.Minute):
	}

	descDsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=kube-system",
		"describe",
		"daemonset.apps/nvidia-device-plugin-daemonset",
	}
	descDsCmd := strings.Join(descDsArgs, " ")

	descPoArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=default",
		"describe",
		"pod/nvidia-smi",
	}
	descPoCmd := strings.Join(descPoArgs, " ")

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=default",
		"logs",
		"nvidia-smi",
		"--timestamps",
	}
	logsCmd := strings.Join(logsArgs, " ")

	installed := false
	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("nvidia-smi check aborted")
		case <-time.After(5 * time.Second):
		}
		ts.cfg.Logger.Info("querying nvidia-smi")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := exec.New().CommandContext(ctx, descDsArgs[0], descDsArgs[1:]...).CombinedOutput()
		cancel()
		output := strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe daemonset.apps/nvidia-device-plugin-daemonset", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descDsCmd, output)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		out, err = exec.New().CommandContext(ctx, descPoArgs[0], descPoArgs[1:]...).CombinedOutput()
		cancel()
		output = strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe pod/nvidia-smi", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", descPoCmd, output)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		out, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		output = strings.TrimSpace(string(out))
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl logs", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", logsCmd, output)

		if strings.Contains(output, "NVIDIA-SMI") && strings.Contains(output, "GPU-Util") {
			installed = true
			break
		}
	}

	if installed {
		ts.cfg.Logger.Info("checked nvidia-smi")
		ts.cfg.EKSConfig.Sync()
		return nil
	}
	ts.cfg.Logger.Warn("failed to test nvidia-smi")
	return errors.New("nvidia-smi failed")
}
