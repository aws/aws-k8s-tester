// Package cudavectoradd implements tester for CUDA GPU Test.
package cudavectoradd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Cuda-Vector-Add configuration.
type Config struct {
	Logger    *zap.Logger
	LogWriter io.Writer
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

const (
	podName = "cuda-vector-add"
	appName = "cuda-vector-add"
)

var pkgName = reflect.TypeOf(tester{}).PkgPath()

func (ts *tester) Name() string { return pkgName }

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", pkgName))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCUDAVectorAdd() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", pkgName))
		return nil
	}

	ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCUDAVectorAdd.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace,
	); err != nil {
		return err
	}
	if err = ts.createPod(); err != nil {
		return err
	}
	if err = ts.checkPod(); err != nil {
		return err
	}

	ts.cfg.Logger.Info("successfully created Pod", zap.String("pod-name", podName))
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) createPod() (err error) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyOnFailure,
			Containers: []v1.Container{
				{
					Name: appName,
					// https://github.com/kubernetes/kubernetes/blob/v1.7.11/test/images/nvidia-cuda/Dockerfile
					Image: "k8s.gcr.io/cuda-vector-add:v0.1",
					Resources: v1.ResourceRequirements{
						Limits: map[v1.ResourceName]resource.Quantity{
							v1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
						},
					},
				},
			},
		},
	}

	ts.cfg.Logger.Info("creating Pod", zap.String("pod-name", podName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.
		KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace).
		Create(ctx, pod, metav1.CreateOptions{})
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Pod (%v)", err)
	}

	ts.cfg.Logger.Info("created Pod")
	ts.cfg.EKSConfig.Sync()
	return nil
}

func (ts *tester) checkPod() error {
	descArgsPods := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace,
		"describe",
		"pods/" + podName,
	}
	descCmdPods := strings.Join(descArgsPods, " ")

	logArgs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace=" + ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace,
		"logs",
		"pods/" + podName,
		"--all-containers=true",
		"--timestamps",
	}
	logsCmd := strings.Join(logArgs, " ")

	ts.cfg.Logger.Info("checking Pod",
		zap.String("pod-name", podName),
		zap.String("container-name", appName),
		zap.String("command-describe", descCmdPods),
		zap.String("command-logs", logsCmd),
	)

	succeeded := false
	retryStart, waitDur := time.Now(), 5*time.Minute
	for time.Since(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("cuda-vector-add pod check aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, err := exec.New().CommandContext(ctx, descArgsPods[0], descArgsPods[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", descCmdPods, out)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		output, err = exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		}
		fmt.Fprintf(ts.cfg.LogWriter, "\n'%s' output:\n\n%s\n\n", logsCmd, out)

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		pout, err := ts.cfg.K8SClient.
			KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace).
			Get(
				ctx,
				podName,
				metav1.GetOptions{},
			)
		cancel()
		if err != nil {
			ts.cfg.Logger.Info("failed to query Pod", zap.String("pod-name", podName), zap.Error(err))
			continue
		}
		if pout.Status.Phase != v1.PodSucceeded {
			ts.cfg.Logger.Warn("unexpected Pod phase", zap.String("pod-name", podName), zap.String("pod-phase", fmt.Sprintf("%v", pout.Status.Phase)))
			time.Sleep(5 * time.Second)
			continue
		}
		if !strings.Contains(out, "Test PASSED") || !strings.Contains(out, "[Vector addition") {
			ts.cfg.Logger.Warn("unexpected logs output")
			time.Sleep(5 * time.Second)
			continue
		}

		succeeded = true
		ts.cfg.Logger.Info("successfully checked Pod logs",
			zap.String("pod-name", podName),
			zap.String("container-name", appName),
		)
		break
	}

	if !succeeded {
		return fmt.Errorf("failed to created Pod %q", podName)
	}
	ts.cfg.EKSConfig.Sync()
	return nil
}

/*
  Type    Reason     Age   From                                                   Message
  ----    ------     ----  ----                                                   -------
  Normal  Scheduled  40s   default-scheduler                                      Successfully assigned eks-2020060315-bluesbw5mxnq-cuda-vector-add/cuda-vector-add to ip-192-168-93-246.us-west-2.compute.internal
  Normal  Pulling    39s   kubelet, ip-192-168-93-246.us-west-2.compute.internal  Pulling image "k8s.gcr.io/cuda-vector-add:v0.1"
  Normal  Pulled     18s   kubelet, ip-192-168-93-246.us-west-2.compute.internal  Successfully pulled image "k8s.gcr.io/cuda-vector-add:v0.1"
  Normal  Created    4s    kubelet, ip-192-168-93-246.us-west-2.compute.internal  Created container cuda-vector-add
  Normal  Started    3s    kubelet, ip-192-168-93-246.us-west-2.compute.internal  Started container cuda-vector-add


'/tmp/kubectl-test-v1.16.9 --kubeconfig=/tmp/leegyuho-test-eks.kubeconfig.yaml --namespace=eks-2020060315-bluesbw5mxnq-cuda-vector-add logs pods/cuda-vector-add --timestamps --all-containers=true' output:

2020-06-03T22:47:36.676558366Z [Vector addition of 50000 elements]
2020-06-03T22:47:36.676594518Z Copy input data from the host memory to the CUDA device
2020-06-03T22:47:36.676600024Z CUDA kernel launch with 196 blocks of 256 threads
2020-06-03T22:47:36.676604025Z Copy output data from the CUDA device to the host memory
2020-06-03T22:47:36.67660785Z Test PASSED
2020-06-03T22:47:36.676611883Z Done

Ref: https://docs.deep-hybrid-datacloud.eu/en/latest/technical/kubernetes/gpu-kubernetes-centos7.html#span-style-color-rgb-0-0-0-text-decoration-none-test-1-span-style-color-rgb-0-0-0-text-decoration-none-simple-vector-add-cuda8-span-span
It takes < 10 seconds to complete

$ kubectl apply -f vector-add.yaml
pod "vector-add" created

$ kubectl get pods/vector-add
NAME                   READY     STATUS      RESTARTS   AGE
vector-add       0/1       Completed   0     4s
*/

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCUDAVectorAdd() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", pkgName))
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCUDAVectorAdd.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	ts.cfg.Logger.Info("deleting Pod", zap.String("pod-name", podName))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.
		KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace).
		Delete(
			ctx,
			podName,
			metav1.DeleteOptions{},
		)
	cancel()
	if err != nil && !apierrs.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return fmt.Errorf("failed to delete Pod (%v)", err)
	}
	ts.cfg.Logger.Info("deleted Pod", zap.String("pod-name", podName), zap.Error(err))

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
		k8s_client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete pod CudaVectorAdd namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCUDAVectorAdd.Created = false
	ts.cfg.EKSConfig.Sync()
	return nil
}
