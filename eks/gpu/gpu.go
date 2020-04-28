// Package gpu implements GPU plugin.
package gpu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"go.uber.org/zap"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines GPU configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines GPU tester.
type Tester interface {
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

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

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

	retryStart, waitDur := time.Now(), 3*time.Minute
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("install nvidia GPU driver stopped")
			return nil
		case <-time.After(5 * time.Second):
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		_, err = ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			DaemonSets("kube-system").
			Create(
				ctx,
				&apps_v1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apps/v1",
						Kind:       "DaemonSet",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nvidia-device-plugin-daemonset",
						Namespace: "kube-system",
					},
					Spec: apps_v1.DaemonSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"name": "nvidia-device-plugin-ds",
							},
						},
						UpdateStrategy: apps_v1.DaemonSetUpdateStrategy{
							Type: apps_v1.RollingUpdateDaemonSetStrategyType,
						},
						Template: v1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"name": "nvidia-device-plugin-ds",
								},
							},
							Spec: v1.PodSpec{
								Tolerations: []v1.Toleration{
									{
										Key:      "nvidia.com/gpu",
										Operator: v1.TolerationOpExists,
										Effect:   v1.TaintEffectNoSchedule,
									},
								},
								PriorityClassName: "system-node-critical",
								Containers: []v1.Container{
									{
										Image: "nvidia/k8s-device-plugin:1.0.0-beta5",
										Name:  "nvidia-device-plugin-ctr",
										SecurityContext: &v1.SecurityContext{
											AllowPrivilegeEscalation: aws.Bool(false),
											Capabilities: &v1.Capabilities{
												Drop: []v1.Capability{v1.Capability("ALL")},
											},
										},
									},
								},
								Volumes: []v1.Volume{
									{
										Name: "device-plugin",
										VolumeSource: v1.VolumeSource{
											HostPath: &v1.HostPathVolumeSource{
												Path: "/var/lib/kubelet/device-plugins",
											},
										},
									},
								},
							},
						},
					},
				},
				metav1.CreateOptions{},
			)
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to create nvidia GPU driver", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		ts.cfg.Logger.Info("created nvidia GPU driver")
		break
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
		for time.Now().Sub(retryStart) < waitDur {
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
					if labels["Name"] != ngName {
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
					zap.Int64("current-ready-nodes", foundReady),
					zap.Int64("desired-ready-nodes", cur.ASGDesiredCapacity),
				)
				time.Sleep(5 * time.Second)

				if foundReady >= cur.ASGDesiredCapacity {
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
		for time.Now().Sub(retryStart) < waitDur {
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
					if labels["Name"] != mngName {
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
					zap.Int("desired-ready-nodes", cur.ASGDesiredCapacity),
				)
				time.Sleep(5 * time.Second)

				if foundReady >= cur.ASGDesiredCapacity {
					readyMNGs[mngName] = struct{}{}
					break
				}
			}
		}
	}

	return ts.cfg.EKSConfig.Sync()
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
	ts.cfg.Logger.Info("creating nvidia-smi")
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
					NodeSelector: map[string]string{
						"AMIType": ec2config.AMITypeAL2X8664GPU,
					},
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

	descArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=default",
		"describe",
		"po",
		"nvidia-smi",
	}
	descCmd := strings.Join(descArgs, " ")

	logsArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=default",
		"logs",
		"nvidia-smi",
	}
	logsCmd := strings.Join(logsArgs, " ")

	waitDur := 2 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("nvidia-smi check aborted")
		case <-time.After(5 * time.Second):
		}
		ts.cfg.Logger.Info("querying nvidia-smi Pod")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := exec.New().CommandContext(ctx, descArgs[0], descArgs[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl describe", zap.Error(err))
		}
		fmt.Printf("\n\n'%s' output:\n\n%s\n\n", descCmd, string(out))

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		out, err = exec.New().CommandContext(ctx, logsArgs[0], logsArgs[1:]...).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to kubectl logs", zap.Error(err))
		}
		fmt.Printf("\n\n'%s' output:\n\n%s\n\n", logsCmd, string(out))

		if strings.Contains(string(out), "GPU-Util") {
			break
		}
	}

	ts.cfg.Logger.Info("checked nvidia-smi")
	return ts.cfg.EKSConfig.Sync()
}
