// Package gpu implements GPU plugin.
package gpu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
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
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	Namespace string
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
	// RunNvidiaSMI launches a pod manifest that launches a Cuda container that
	// runs "nvidia-smi" on a GPU worker node.
	// ref. https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
	RunNvidiaSMI() error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/eks/latest/userguide/gpu-ami.html
// ref. https://github.com/NVIDIA/k8s-device-plugin
// ref. https://github.com/NVIDIA/k8s-device-plugin/releases
func (ts *tester) InstallNvidiaDriver() error {
	ts.cfg.Logger.Info("applying daemon set for Nvidia GPU driver for worker nodes")
	downloadURL := "https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/1.0.0-beta4/nvidia-device-plugin.yml"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	out, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"apply",
		"-f",
		downloadURL,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl apply' failed (output %q, error %v)", string(out), err)
	}
	ts.cfg.Logger.Info("applied daemon set for Nvidia GPU driver for worker nodes", zap.String("output", string(out)))

	waitDur := 5 * time.Minute
	var items []v1.Node
	retryStart := time.Now()

	readyMNGs := make(map[string]struct{})
	for time.Now().Sub(retryStart) < waitDur {
		for _, mv := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
			switch mv.AMIType {
			case eks.AMITypesAl2X8664Gpu:
			default:
				continue
			}
			if _, ok := readyMNGs[mv.Name]; ok {
				continue
			}

			ts.cfg.Logger.Info("listing GPU nodes via client-go", zap.String("mng-name", mv.Name))
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
			ts.cfg.Logger.Info("listed GPU nodes via client-go", zap.String("mng-name", mv.Name), zap.Int("nodes", len(items)))

			readies := 0
			for _, node := range items {
				for _, cond := range node.Status.Conditions {
					if cond.Type != v1.NodeReady {
						continue
					}
					ts.cfg.Logger.Info("node info",
						zap.String("name", node.GetName()),
						zap.String("type", fmt.Sprintf("%s", cond.Type)),
						zap.String("status", fmt.Sprintf("%s", cond.Status)),
					)
					if cond.Status == v1.ConditionTrue {
						readies++
					}
				}
			}
			ts.cfg.Logger.Info("nodes",
				zap.Int("current-ready-nodes", readies),
				zap.Int("desired-ready-nodes", mv.ASGDesiredCapacity),
			)

			if mv.ASGDesiredCapacity <= readies {
				readyMNGs[mv.Name] = struct{}{}
				break
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
func (ts *tester) RunNvidiaSMI() error {
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
	case sig := <-ts.cfg.Sig:
		return fmt.Errorf("received os signal %v", sig)
	case <-time.After(time.Minute):
	}

	waitDur := 2 * time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("nvidia-smi check aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}
		ts.cfg.Logger.Info("querying nvidia-smi logs")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"logs",
			"nvidia-smi",
		).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to fetch logs", zap.Error(err))
		}
		fmt.Println("nvidia-smi output:", string(out))

		if strings.Contains(string(out), "GPU-Util") {
			break
		}
	}

	ts.cfg.Logger.Info("checked nvidia-smi")
	return ts.cfg.EKSConfig.Sync()
}
