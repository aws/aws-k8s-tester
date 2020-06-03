// Package cudavector implements tester for CUDA GPU Test.
package cudavector

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines Cuda-Vector-Add configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

const (
	cuda_vector_add_pod_name = "vector-add"
)

// Tester defines Cuda-Vector-Add tester.
type Tester interface {
	// Create method deploy Cuda-Vector-Add.
	Create() error
	// Delete method delete Cuda-Vector-Add.
	Delete() error
}

func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCudaVectorAdd() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnCudaVectorAdd.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.EKSConfig.AddOnCudaVectorAdd.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCudaVectorAdd.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace,
	); err != nil {
		return err
	}

	ts.cfg.Logger.Info("creating pod",
		zap.String("name", "CudaVectorAdd"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.
		KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace).
		Create(
			ctx,
			&v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: cuda_vector_add_pod_name,
				},
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name: "cuda-vector-add",
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
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create Pod (%v)", err)
	}

	retryStart, waitDur := time.Now(), 3*time.Minute

	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("cuda-vector-add pod check aborted")
		case <-time.After(5 * time.Second):
		}

		ts.cfg.Logger.Info("querying cuda-vector-add")
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		out, err := ts.cfg.K8SClient.
			KubernetesClientSet().
			CoreV1().
			Pods(ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace).
			Get(
				ctx,
				"vector-add",
				metav1.GetOptions{},
			)
		cancel()
		if err != nil || out.Status.Phase == "Succeeded" {
			ts.cfg.Logger.Info("Cuda-vector-add still running")
			continue
		}
		logArgs := []string{
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace=",
			ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace,
			"logs",
			"pod/",
			cuda_vector_add_pod_name,
			"--all-containers=true",
		}
		logsCmd := strings.Join(logArgs, " ")

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		logsOutput, err := exec.New().CommandContext(ctx, logArgs[0], logArgs[1:]...).CombinedOutput()
		cancel()
		logsOut := strings.TrimSpace(string(logsOutput))
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl logs' failed", zap.Error(err))
		} else {
			fmt.Printf("\n\n'%s' output:\n\n%s\n\n", logsCmd, logsOut)
		}

		ts.cfg.Logger.Info("Cuda-vector-add succeeded")
		break
	}

	ts.cfg.Logger.Info("created Cuda Vector Add Pod")

	return ts.cfg.EKSConfig.Sync()
}

/*
Ref: https://docs.deep-hybrid-datacloud.eu/en/latest/technical/kubernetes/gpu-kubernetes-centos7.html#span-style-color-rgb-0-0-0-text-decoration-none-test-1-span-style-color-rgb-0-0-0-text-decoration-none-simple-vector-add-cuda8-span-span
It takes < 10 seconds to complete

$ kubectl apply -f vector-add.yaml
pod "vector-add" created

$ kubectl get pods/vector-add
NAME                   READY     STATUS      RESTARTS   AGE
vector-add       0/1       Completed   0     4s
*/

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCudaVectorAdd() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCudaVectorAdd.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCudaVectorAdd.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	ts.cfg.Logger.Info("deleting pod",
		zap.String("name", "CudaVectorAdd"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	err := ts.cfg.K8SClient.
		KubernetesClientSet().
		CoreV1().
		Pods(ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace).
		Delete(
			ctx,
			cuda_vector_add_pod_name,
			metav1.DeleteOptions{},
		)

	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete Pod (%v)", err)
	}

	ts.cfg.Logger.Info("deleted Cuda Vector Add Pod")

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnCudaVectorAdd.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete pod CudaVectorAdd namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCudaVectorAdd.Created = false

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnCudaVectorAdd() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnCudaVectorAdd.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return nil
}