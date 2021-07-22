package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s_client "k8s.io/client-go/kubernetes"
)

var errPodCompleted = fmt.Errorf("pod ran to completion")

type podCondition func(pod *v1.Pod) (bool, error)

const (
	// poll is how often to poll pods, nodes and claims.
	poll = 2 * time.Second

	// podStartTimeout is how long to wait for the pod to be started.
	podStartTimeout = 2 * time.Minute
)

func ListPods(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	batchLimit int64,
	batchInterval time.Duration,
	opts ...OpOption) (pods []core_v1.Pod, err error) {
	ns, err := listPods(lg, c, namespace, batchLimit, batchInterval, 5, opts...)
	return ns, err
}

func listPods(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	batchLimit int64,
	batchInterval time.Duration,
	retryLeft int,
	opts ...OpOption) (pods []core_v1.Pod, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	lg.Info("listing pods",
		zap.String("namespace", namespace),
		zap.Int64("batch-limit", batchLimit),
		zap.Duration("batch-interval", batchInterval),
		zap.String("label-selector", ret.labelSelector),
		zap.String("field-selector", ret.fieldSelector),
	)
	rs := &core_v1.PodList{ListMeta: meta_v1.ListMeta{Continue: ""}}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err = c.CoreV1().Pods(namespace).List(ctx, meta_v1.ListOptions{Limit: batchLimit, Continue: rs.Continue})
		cancel()
		if err != nil {
			if retryLeft > 0 &&
				!IsRetryableAPIError(err) &&
				(strings.Contains(err.Error(), "too old to display a consistent") ||
					strings.Contains(err.Error(), "inconsistent")) {
				// e.g. The provided continue parameter is too old to display a consistent list result. You can start a new list without the continue parameter, or use the continue token in this response to retrieve the remainder of the results. Continuing with the provided token results in an inconsistent list - objects that were created, modified, or deleted between the time the first chunk was returned and now may show up in the list.
				lg.Warn("stale list response, retrying for consistent list", zap.Error(err))
				time.Sleep(15 * time.Second)
				return listPods(lg, c, namespace, batchLimit, batchInterval, retryLeft-1, opts...)
			}
			return nil, err
		}
		pods = append(pods, rs.Items...)
		remained := int64Value(rs.RemainingItemCount)
		lg.Info("pods",
			zap.Int64("remained", remained),
			zap.Int("items", len(rs.Items)),
		)
		if rs.Continue == "" {
			break
		}
		time.Sleep(batchInterval)
	}
	lg.Info("listed pods", zap.Int("pods", len(pods)))
	return pods, err
}

//Checks the last 100 lines of logs from a pod
func CheckPodLogs(
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	c k8s_client.Interface,
	namespace string,
	podName string,
	opts ...OpOption) (logs string, err error) {
	ret := Op{}
	ret.applyOpts(opts)
	lg.Info("waiting pod logs",
		zap.String("namespace", namespace),
		zap.String("pod-name", podName),
	)
	count := int64(100)
	podLogOptions := v1.PodLogOptions{
		TailLines: &count,
	}
	podLogRequest := c.CoreV1().
		Pods(namespace).
		GetLogs(podName, &podLogOptions)
	podLogs, err := podLogRequest.Stream(context.TODO())
	if err != nil {
		lg.Warn("failed to get Pod logs", zap.String("namespace", namespace), zap.String("name", podName), zap.Error(err))
		return "", err
	}
	defer podLogs.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		lg.Warn("failed to copy Pod logs to buffer", zap.String("namespace", namespace), zap.String("name", podName), zap.Error(err))
		return "", err
	}
	templogs := buf.String()
	if err != nil {
		lg.Warn("kubectl logs pods' failed", zap.Error(err))
		return "", err
	}
	return templogs, nil
}

// DeletePod deletes Pod with given name.
func DeletePod(lg *zap.Logger, c k8s_client.Interface, namespace string, Name string) error {
	deleteFunc := func() error {
		lg.Info("deleting Pod", zap.String("namespace", namespace), zap.String("name", Name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			CoreV1().
			Pods(namespace).
			Delete(
				ctx,
				Name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted Pod", zap.String("namespace", namespace), zap.String("name", Name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("Pod already deleted", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete Pod", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// 3
func PodRunning(c k8s_client.Interface, podName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.CoreV1().Pods(namespace).Get(context.TODO(), podName, meta_v1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case v1.PodRunning:
			return true, nil
		case v1.PodFailed, v1.PodSucceeded:
			return false, errPodCompleted
		}
		return false, nil
	}
}

// 2
// WaitTimeoutForPodRunningInNamespace waits the given timeout duration for the specified pod to become running.
func WaitTimeoutForPodRunningInNamespace(c k8s_client.Interface, podName, namespace string, timeout time.Duration) error {
	return wait.PollImmediate(poll, timeout, PodRunning(c, podName, namespace))
}

// 1
// WaitForPodRunningInNamespace waits default amount of time (podStartTimeout) for the specified pod to become running.
// Returns an error if timeout occurs first, or pod goes in to failed state.
func WaitForPodRunningInNamespace(c k8s_client.Interface, pod *v1.Pod) error {
	if pod.Status.Phase == v1.PodRunning {
		return nil
	}
	return WaitTimeoutForPodRunningInNamespace(c, pod.Name, pod.Namespace, podStartTimeout)
}

// WaitForPodCondition waits a pods to be matched to the given condition.
func WaitForPodCondition(lg *zap.Logger, c k8s_client.Interface, ns, podName, desc string, timeout time.Duration, condition podCondition) error {
	lg.Info("Waiting for pod namespace to be desired condition",
		zap.Any("timeout", timeout),
		zap.String("Podname", podName),
		zap.String("namespace", ns),
	)
	var lastPodError error
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(poll) {
		pod, err := c.CoreV1().Pods(ns).Get(context.TODO(), podName, meta_v1.GetOptions{})
		lastPodError = err
		if err != nil {
			lg.Warn("Pod in namespace not found. Error", zap.String("podname", podName), zap.String("namespace", ns))
			return fmt.Errorf("Pod in namespace not found. Error", timeout, podName, desc)
		}
		phase := pod.Status.Phase
		switch phase {
		case "Succeeded":
			return nil
		case "Failed":
			return nil
		default:
			lg.Warn("Pod condition Succeeded/Failed not satisfied yet for", zap.String("podname", podName))
			continue
		}
	}
	if apierrors.IsNotFound(lastPodError) {
		// return for compatbility with other functions testing for IsNotFound
		return lastPodError
	}
	return fmt.Errorf("Gave up after waiting %v for pod %q to be %q", timeout, podName, desc)
}

// WaitForPodSuccessInNamespaceTimeout returns nil if the pod reached state success, or an error if it reached failure or ran too long.
func WaitForPodSuccessInNamespaceTimeout(lg *zap.Logger, c k8s_client.Interface, podName, namespace string, timeout time.Duration) error {
	return WaitForPodCondition(lg, c, namespace, podName, fmt.Sprintf("%s or %s", v1.PodSucceeded, v1.PodFailed), timeout, func(pod *v1.Pod) (bool, error) {
		if pod.Spec.RestartPolicy == v1.RestartPolicyAlways {
			return true, fmt.Errorf("pod %q will never terminate with a succeeded state since its restart policy is Always", podName)
		}
		switch pod.Status.Phase {
		case v1.PodSucceeded:
			ginkgo.By("Saw pod success")
			return true, nil
		case v1.PodFailed:
			return true, fmt.Errorf("pod %q failed with status: %+v", podName, pod.Status)
		default:
			return false, nil
		}
	})
}

func NewBusyBoxPod(name, command string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "public.ecr.aws/hudsonbay/busybox:latest",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", command},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}
