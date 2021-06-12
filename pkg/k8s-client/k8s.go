/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
https://github.com/kubernetes/perf-tests/blob/master/clusterloader2/pkg/framework/client/objects.go
https://github.com/kubernetes/kubernetes/blob/master/cmd/kubeadm/app/util/apiclient/wait.go#L49
*/

package k8sclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/ctxutil"
	"github.com/aws/aws-k8s-tester/pkg/spinner"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// Parameters for retrying with exponential backoff.
	retryBackoffInitialDuration = 100 * time.Millisecond
	retryBackoffFactor          = 3
	retryBackoffJitter          = 0
	retryBackoffSteps           = 6

	// DefaultNamespacePollInterval is the default namespace poll interval.
	DefaultNamespacePollInterval = 15 * time.Second
	// DefaultNamespaceDeletionInterval is the default namespace deletion interval.
	DefaultNamespaceDeletionInterval = 15 * time.Second
	// DefaultNamespaceDeletionTimeout is the default namespace deletion timeout.
	DefaultNamespaceDeletionTimeout = 30 * time.Minute
)

// RetryWithExponentialBackOff a utility for retrying the given function with exponential backoff.
func RetryWithExponentialBackOff(fn wait.ConditionFunc) error {
	backoff := wait.Backoff{
		Duration: retryBackoffInitialDuration,
		Factor:   retryBackoffFactor,
		Jitter:   retryBackoffJitter,
		Steps:    retryBackoffSteps,
	}
	return wait.ExponentialBackoff(backoff, fn)
}

// IsRetryableAPIError verifies whether the error is retryable.
func IsRetryableAPIError(err error) bool {
	// These errors may indicate a transient error that we can retry in tests.
	if apierrs.IsInternalError(err) || apierrs.IsTimeout(err) || apierrs.IsServerTimeout(err) ||
		apierrs.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) ||
		// Retryable resource-quotas conflict errors may be returned in some cases, e.g. https://github.com/kubernetes/kubernetes/issues/67761
		isResourceQuotaConflictError(err) ||
		// Our client is using OAuth2 where 401 (unauthorized) can mean that our token has expired and we need to retry with a new one.
		apierrs.IsUnauthorized(err) {
		return true
	}
	// If the error sends the Retry-After header, we respect it as an explicit confirmation we should retry.
	if _, shouldRetry := apierrs.SuggestsClientDelay(err); shouldRetry {
		return true
	}
	return false
}

func isResourceQuotaConflictError(err error) bool {
	apiErr, ok := err.(apierrs.APIStatus)
	if !ok {
		return false
	}
	if apiErr.Status().Reason != metav1.StatusReasonConflict {
		return false
	}
	return apiErr.Status().Details != nil && apiErr.Status().Details.Kind == "resourcequotas"
}

// IsRetryableNetError determines whether the error is a retryable net error.
func IsRetryableNetError(err error) bool {
	if netError, ok := err.(net.Error); ok {
		return netError.Temporary() || netError.Timeout()
	}
	return false
}

// ApiCallOptions describes how api call errors should be treated, i.e. which errors should be
// allowed (ignored) and which should be retried.
type ApiCallOptions struct {
	shouldAllowError func(error) bool
	shouldRetryError func(error) bool
}

// Allow creates an ApiCallOptions that allows (ignores) errors matching the given predicate.
func Allow(allowErrorPredicate func(error) bool) *ApiCallOptions {
	return &ApiCallOptions{shouldAllowError: allowErrorPredicate}
}

// Retry creates an ApiCallOptions that retries errors matching the given predicate.
func Retry(retryErrorPredicate func(error) bool) *ApiCallOptions {
	return &ApiCallOptions{shouldRetryError: retryErrorPredicate}
}

// RetryFunction opaques given function into retryable function.
func RetryFunction(f func() error, options ...*ApiCallOptions) wait.ConditionFunc {
	var shouldAllowErrorFuncs, shouldRetryErrorFuncs []func(error) bool
	for _, option := range options {
		if option.shouldAllowError != nil {
			shouldAllowErrorFuncs = append(shouldAllowErrorFuncs, option.shouldAllowError)
		}
		if option.shouldRetryError != nil {
			shouldRetryErrorFuncs = append(shouldRetryErrorFuncs, option.shouldRetryError)
		}
	}
	return func() (bool, error) {
		err := f()
		if err == nil {
			return true, nil
		}
		if IsRetryableAPIError(err) || IsRetryableNetError(err) {
			return false, nil
		}
		for _, shouldAllowError := range shouldAllowErrorFuncs {
			if shouldAllowError(err) {
				return true, nil
			}
		}
		for _, shouldRetryError := range shouldRetryErrorFuncs {
			if shouldRetryError(err) {
				return false, nil
			}
		}
		return false, err
	}
}

// ListPodsWithOptions lists the pods using the provided options.
func ListPodsWithOptions(c clientset.Interface, namespace string, listOpts metav1.ListOptions) ([]v1.Pod, error) {
	var pods []v1.Pod
	listFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		podsList, err := c.CoreV1().Pods(namespace).List(ctx, listOpts)
		cancel()
		if err != nil {
			return err
		}
		pods = podsList.Items
		return nil
	}
	if err := RetryWithExponentialBackOff(RetryFunction(listFunc)); err != nil {
		return pods, err
	}
	return pods, nil
}

// ListNodes returns list of cluster nodes.
func ListNodes(c clientset.Interface) ([]v1.Node, error) {
	return ListNodesWithOptions(c, metav1.ListOptions{})
}

// ListNodesWithOptions lists the cluster nodes using the provided options.
func ListNodesWithOptions(c clientset.Interface, listOpts metav1.ListOptions) ([]v1.Node, error) {
	var nodes []v1.Node
	listFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodesList, err := c.CoreV1().Nodes().List(ctx, listOpts)
		cancel()
		if err != nil {
			return err
		}
		nodes = nodesList.Items
		return nil
	}
	if err := RetryWithExponentialBackOff(RetryFunction(listFunc)); err != nil {
		return nodes, err
	}
	return nodes, nil
}

// CreateNamespace creates a single namespace with given name.
func CreateNamespace(lg *zap.Logger, c clientset.Interface, namespace string) error {
	createFunc := func() error {
		lg.Info("creating namespace", zap.String("namespace", namespace))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := c.CoreV1().Namespaces().Create(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
		cancel()
		if err == nil {
			lg.Info("created namespace", zap.String("namespace", namespace))
			return nil
		}
		if apierrs.IsAlreadyExists(err) {
			lg.Info("namespace already exists", zap.String("namespace", namespace), zap.Error(err))
			return nil
		}
		lg.Warn("failed to create namespace", zap.String("namespace", namespace), zap.Error(err))
		return err
	}
	return RetryWithExponentialBackOff(RetryFunction(createFunc, Allow(apierrs.IsAlreadyExists)))
}

// DeleteNamespaceAndWait deletes namespace with given name and waits for its deletion.
// Default interval is 5-second and default timeout is 10-min.
func DeleteNamespaceAndWait(
	lg *zap.Logger,
	c clientset.Interface,
	namespace string,
	pollInterval time.Duration,
	timeout time.Duration,
	opts ...OpOption) error {
	if err := deleteNamespace(lg, c, namespace); err != nil {
		return err
	}
	return waitForDeleteNamespace(lg, c, namespace, pollInterval, timeout, opts...)
}

// deleteNamespace deletes namespace with given name.
func deleteNamespace(lg *zap.Logger, c clientset.Interface, namespace string) error {
	foreground, zero := metav1.DeletePropagationForeground, int64(0)
	deleteFunc := func() error {
		lg.Info("deleting namespace", zap.String("namespace", namespace))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.CoreV1().Namespaces().Delete(
			ctx,
			namespace,
			metav1.DeleteOptions{
				GracePeriodSeconds: &zero,
				PropagationPolicy:  &foreground,
			},
		)
		cancel()
		if err == nil {
			lg.Info("deleted namespace", zap.String("namespace", namespace))
			return nil
		}
		if apierrs.IsNotFound(err) || apierrs.IsGone(err) {
			lg.Info("namespace already deleted", zap.String("namespace", namespace), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete namespace", zap.String("namespace", namespace), zap.Error(err))
		return err
	}
	// requires "apierrs.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(apierrs.IsNotFound)))
}

func waitForDeleteNamespace(lg *zap.Logger, c clientset.Interface, namespace string, pollInterval time.Duration, timeout time.Duration, opts ...OpOption) error {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespaceDeletionInterval
	}
	if timeout == 0 {
		timeout = DefaultNamespaceDeletionTimeout
	}

	retryWaitFunc := func() (done bool, err error) {
		lg.Info("waiting for namespace deletion", zap.String("namespace", namespace))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		var ns *v1.Namespace
		ns, err = c.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		cancel()
		if err != nil {
			if apierrs.IsNotFound(err) {
				lg.Info("namespace already deleted", zap.String("namespace", namespace))
				return true, nil
			}
			lg.Warn("failed to get namespace", zap.String("namespace", namespace), zap.Error(err))
			if strings.Contains(err.Error(), "i/o timeout") {
				return false, nil
			}
			if !IsRetryableAPIError(err) {
				return false, err
			}
		}
		lg.Info("namespace still exists", zap.String("namespace", namespace))

		if ret.queryFunc != nil {
			ret.queryFunc()
		}

		finalizers := ns.GetFinalizers()
		if ret.forceDelete && len(finalizers) > 0 {
			lg.Warn("deleting namespace finalizers to force-delete",
				zap.String("namespace", namespace),
				zap.Strings("finalizers", finalizers),
			)
			ns.SetFinalizers(nil)

			ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
			_, err = c.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
			cancel()
			if err == nil {
				lg.Info("deleted namespace finalizers",
					zap.String("namespace", namespace),
					zap.Strings("finalizers", finalizers),
				)
			} else {
				lg.Warn("failed to delete namespace finalizers",
					zap.String("namespace", namespace),
					zap.Strings("finalizers", finalizers),
					zap.Error(err),
				)
			}
		}

		if ret.forceDeleteFunc != nil {
			ret.forceDeleteFunc()
		}

		return false, nil
	}
	return wait.PollImmediate(pollInterval, timeout, retryWaitFunc)
}

/*
k8s_client.WithForceDeleteFunc(forceDeleteFunc),

var forceDeleteFunc func()
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
ns, err := ts.cfg.K8SClient.
	KubernetesClientSet().
	CoreV1().
	Namespaces().
	Get(ctx, ts.cfg.EKSConfig.AddOnStresserRemote.Namespace, metav1.GetOptions{})
cancel()
if err != nil {
	lg.Warn("failed to get namespace", zap.Error(err))
} else {
	ns.SetFinalizers(nil)
	jb, jerr := json.Marshal(ns)
	if jerr != nil {
		lg.Warn("failed to marshal JSON", zap.Error(jerr))
	} else {
		jpath, err := fileutil.WriteTempFile(jb)
		if err != nil {
			lg.Warn("failed to write JSON", zap.Error(err))
		} else {
			target := fmt.Sprintf("/api/v1/namespaces/%s/finalize", ts.cfg.EKSConfig.AddOnStresserRemote.Namespace)
			replaceArgs := []string{
				ts.cfg.EKSConfig.KubectlPath,
				"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
				"--namespace=" + ts.cfg.EKSConfig.AddOnStresserRemote.Namespace,
				"replace",
				"--raw",
				target,
				"--force",
				"--filename=" + jpath,
			}
			replaceCmd := strings.Join(replaceArgs, " ")

			forceDeleteFunc = func() {
				println()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				output, err := exec.New().CommandContext(ctx, replaceArgs[0], replaceArgs[1:]...).CombinedOutput()
				cancel()
				out := strings.TrimSpace(string(output))
				if err != nil {
					lg.Warn("'kubectl replace' failed", zap.Error(err))
				} else {
					fmt.Printf("\n\n'%s' output:\n\n%s\n\n", replaceCmd, out)
					fmt.Printf("\n\n'%s' JSON:\n\n%s\n\n", jpath, string(jb))
				}
			}
		}
	}
}

$ kubectl get namespace "myname" -o json \
	| tr -d "\n" | sed "s/\"finalizers\": \[[^]]\+\]/\"finalizers\": []/" \
	| kubectl replace --raw /api/v1/namespaces/myname/finalize -f -

$ kubectl get ns eks-2020062119-floralb5826l-stresser-remote -o json
{
    "apiVersion": "v1",
    "kind": "Namespace",
    "metadata": {
        "creationTimestamp": "2020-06-22T03:35:15Z",
        "deletionTimestamp": "2020-06-22T04:13:22Z",
        "name": "eks-2020062119-floralb5826l-stresser-remote",
        "resourceVersion": "220505",
        "selfLink": "/api/v1/namespaces/eks-2020062119-floralb5826l-stresser-remote",
        "uid": "eefaada7-0b44-4b54-9772-cab450931468"
    },
    "spec": {
        "finalizers": [
            "kubernetes"
        ]
    },
    "status": {
        "conditions": [
            {
                "lastTransitionTime": "2020-06-22T04:14:35Z",
                "message": "All resources successfully discovered",
                "reason": "ResourcesDiscovered",
                "status": "False",
                "type": "NamespaceDeletionDiscoveryFailure"
            },
            {
                "lastTransitionTime": "2020-06-22T04:14:35Z",
                "message": "All legacy kube types successfully parsed",
                "reason": "ParsedGroupVersions",
                "status": "False",
                "type": "NamespaceDeletionGroupVersionParsingFailure"
            },
            {
                "lastTransitionTime": "2020-06-22T04:14:35Z",
                "message": "Failed to delete all resource types, 1 remaining: Timeout: request did not complete within requested timeout 34s",
                "reason": "ContentDeletionFailed",
                "status": "True",
                "type": "NamespaceDeletionContentFailure"
            },
            {
                "lastTransitionTime": "2020-06-22T04:14:35Z",
                "message": "All content successfully removed",
                "reason": "ContentRemoved",
                "status": "False",
                "type": "NamespaceContentRemaining"
            },
            {
                "lastTransitionTime": "2020-06-22T04:14:35Z",
                "message": "All content-preserving finalizers finished",
                "reason": "ContentHasNoFinalizers",
                "status": "False",
                "type": "NamespaceFinalizersRemaining"
            }
        ],
        "phase": "Terminating"
    }
}
*/

// ListNamespaces returns list of existing namespace names.
func ListNamespaces(c clientset.Interface) ([]v1.Namespace, error) {
	var namespaces []v1.Namespace
	listFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		namespacesList, err := c.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			return err
		}
		namespaces = namespacesList.Items
		return nil
	}
	if err := RetryWithExponentialBackOff(RetryFunction(listFunc)); err != nil {
		return namespaces, err
	}
	return namespaces, nil
}

// ListEvents retrieves events for the object with the given name.
func ListEvents(c clientset.Interface, namespace string, name string, options ...*ApiCallOptions) (obj *v1.EventList, err error) {
	getFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		obj, err = c.CoreV1().Events(namespace).List(
			ctx,
			metav1.ListOptions{
				FieldSelector: "involvedObject.name=" + name,
			},
		)
		cancel()
		return err
	}
	if err := RetryWithExponentialBackOff(RetryFunction(getFunc, options...)); err != nil {
		return nil, err
	}
	return obj, nil
}

// CreateObject creates object based on given object description.
func CreateObject(dynamicClient dynamic.Interface, namespace string, name string, obj *unstructured.Unstructured, options ...*ApiCallOptions) error {
	gvk := obj.GroupVersionKind()
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	obj.SetName(name)
	createFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
		cancel()
		return err
	}
	options = append(options, Allow(apierrs.IsAlreadyExists))
	return RetryWithExponentialBackOff(RetryFunction(createFunc, options...))
}

// WaitForJobCompletes waits for all Job completion,
// by counting the number of pods in the namespace.
func WaitForJobCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	k8sClient EKS,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (job *batchv1.Job, pods []v1.Pod, err error) {
	job, _, pods, err = waitForJobCompletes(false, ctx, lg, logWriter, stopc, k8sClient, initialWait, pollInterval, namespace, jobName, targetCompletes, opts...)
	return job, pods, err
}

// WaitForCronJobCompletes waits for all CronJob completion,
// by counting the number of pods in the namespace.
func WaitForCronJobCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	k8sClient EKS,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (cronJob *batchv1beta1.CronJob, pods []v1.Pod, err error) {
	_, cronJob, pods, err = waitForJobCompletes(true, ctx, lg, logWriter, stopc, k8sClient, initialWait, pollInterval, namespace, jobName, targetCompletes, opts...)
	return cronJob, pods, err
}

/*
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kubernetes.io/psp: eks.privileged
  creationTimestamp: "2020-06-26T21:00:05Z"
  generateName: cronjob-echo-1593205200-
  labels:
    controller-uid: 724164ed-ca62-4468-b7f7-c762dac0ec42
    job-name: cronjob-echo-1593205200
  name: cronjob-echo-1593205200-2t2tv
  namespace: eks-2020062613-rustcerg03pt-cronjob
*/

func waitForJobCompletes(
	isCronJob bool,
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	k8sClient EKS,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (job *batchv1.Job, cronJob *batchv1beta1.CronJob, pods []v1.Pod, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := spinner.New(logWriter, "Waiting for Job completes "+jobName)
	lg.Info("waiting Job completes",
		zap.String("namespace", namespace),
		zap.String("job-name", jobName),
		zap.Bool("cron-job", isCronJob),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", ctxutil.DurationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
		zap.Int("target-completes", targetCompletes),
	)
	sp.Restart()
	select {
	case <-stopc:
		sp.Stop()
		return nil, nil, nil, errors.New("initial wait aborted")
	case <-time.After(initialWait):
		sp.Stop()
	}

	retryWaitFunc := func() (done bool, err error) {
		select {
		case <-stopc:
			return true, errors.New("wait aborted")
		default:
		}

		lg.Info("listing job pods to check Job completion")
		pods, err = k8sClient.ListPods(namespace, 3000, 3*time.Second)
		if err != nil {
			lg.Warn("failed to list Pod", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
			return false, err
		}
		if len(pods) == 0 {
			lg.Warn("got an empty list of Pod")
			if ret.queryFunc != nil {
				ret.queryFunc()
			}
			return false, nil
		}
		podSucceededCnt := 0
		for _, pod := range pods {
			jv, ok := pod.Labels["job-name"]
			match := ok && jv == jobName
			if !match {
				// CronJob
				match = strings.HasPrefix(pod.Name, jobName)
			}
			lg.Info("pod",
				zap.String("job-name", jobName),
				zap.String("job-name-from-label", jv),
				zap.String("pod-name", pod.Name),
				zap.String("pod-status-phase", fmt.Sprintf("%v", pod.Status.Phase)),
				zap.Bool("label-match", match),
			)
			if ret.podFunc != nil {
				ret.podFunc(pod)
			}
			if !match {
				continue
			}
			if pod.Status.Phase == v1.PodSucceeded {
				podSucceededCnt++
			}
		}
		if podSucceededCnt < targetCompletes {
			lg.Warn("poll job pods but not succeeded yet",
				zap.String("namespace", namespace),
				zap.String("job-name", jobName),
				zap.Int("total-pods", len(pods)),
				zap.Int("pod-succeeded-count", podSucceededCnt),
				zap.Int("target-completes", targetCompletes),
				zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
			)
			if ret.queryFunc != nil {
				ret.queryFunc()
			}
			return false, nil
		}
		lg.Info("job pods",
			zap.String("namespace", namespace),
			zap.String("job-name", jobName),
			zap.Int("pod-succeeded-count", podSucceededCnt),
			zap.Int("target-completes", targetCompletes),
			zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
		)

		switch isCronJob {
		case false:
			lg.Info("checking Job object", zap.String("namespace", namespace))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			job, err = k8sClient.KubernetesClientSet().
				BatchV1().
				Jobs(namespace).
				Get(ctx, jobName, metav1.GetOptions{})
			cancel()
			if err != nil {
				lg.Warn("failed to check Job", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
				return false, err
			}
			for _, cond := range job.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type == batchv1.JobFailed {
					lg.Warn("job failed", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
					return true, fmt.Errorf("Job %q status %q", jobName, cond.Type)
				}
				if cond.Type == batchv1.JobComplete {
					lg.Info("job complete", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
					return true, nil
				}
				lg.Warn("job not complete", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
			}

		case true:
			lg.Info("checking CronJob object", zap.String("namespace", namespace))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			cronJob, err = k8sClient.KubernetesClientSet().
				BatchV1beta1().
				CronJobs(namespace).
				Get(ctx, jobName, metav1.GetOptions{})
			cancel()
			if err != nil {
				lg.Warn("failed to check CronJob", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
				return false, err
			}
			lg.Info("checked CronJob object", zap.Int("active-jobs", len(cronJob.Status.Active)))
			return true, nil
		}

		if ret.queryFunc != nil {
			ret.queryFunc()
		}
		return false, nil
	}
	err = wait.PollImmediate(pollInterval, ctxutil.DurationTillDeadline(ctx), retryWaitFunc)
	return job, cronJob, pods, err
}

// WaitForReplicationControllerCompletes waits till target replicas are ready in the ReplicationController.
func WaitForReplicationControllerCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	k8sClient EKS,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	replicationControllerName string,
	targetAvailableReplicas int32,
	opts ...OpOption) (dp *v1.ReplicationController, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := spinner.New(logWriter, "Waiting for ReplicationController completes "+replicationControllerName)
	lg.Info("waiting ReplicationController completes",
		zap.String("namespace", namespace),
		zap.String("replication-controller-name", replicationControllerName),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", ctxutil.DurationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
		zap.Int32("target-available-replicas", targetAvailableReplicas),
	)
	sp.Restart()
	select {
	case <-stopc:
		sp.Stop()
		return nil, errors.New("initial wait aborted")
	case <-time.After(initialWait):
		sp.Stop()
	}

	retryWaitFunc := func() (done bool, err error) {
		select {
		case <-stopc:
			return true, errors.New("wait aborted")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dp, err = k8sClient.KubernetesClientSet().
			CoreV1().
			ReplicationControllers(namespace).
			Get(ctx, replicationControllerName, metav1.GetOptions{})
		cancel()
		if ret.queryFunc != nil {
			ret.queryFunc()
		}
		if err != nil {
			lg.Warn("failed to get ReplicationController", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
			return false, err
		}

		var dpCond v1.ReplicationControllerCondition
		for _, cond := range dp.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				continue
			}
			dpCond = cond
			break
		}
		lg.Info("fetched ReplicationControllers",
			zap.Int32("desired-replicas", dp.Status.Replicas),
			zap.Int32("available-replicas", dp.Status.AvailableReplicas),
			zap.Int32("ready-replicas", dp.Status.ReadyReplicas),
			zap.String("condition-last-updated", dpCond.LastTransitionTime.String()),
			zap.String("condition-type", string(dpCond.Type)),
			zap.String("condition-status", string(dpCond.Status)),
			zap.String("condition-reason", dpCond.Reason),
			zap.String("condition-message", dpCond.Message),
		)
		if dpCond.Type == v1.ReplicationControllerReplicaFailure {
			return true, fmt.Errorf("ReplicationController %q status %q", replicationControllerName, dpCond.Type)
		}
		if dp.Status.AvailableReplicas >= targetAvailableReplicas {
			return true, nil
		}
		return false, nil
	}
	err = wait.PollImmediate(pollInterval, ctxutil.DurationTillDeadline(ctx), retryWaitFunc)
	return dp, err
}

// WaitForDeploymentCompletes waits till target replicas are ready in the Deployment.
func WaitForDeploymentCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	k8sClient EKS,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	deploymentName string,
	targetAvailableReplicas int32,
	opts ...OpOption) (dp *appsv1.Deployment, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := spinner.New(logWriter, "Waiting for Deployment completes "+deploymentName)
	lg.Info("waiting Deployment completes",
		zap.String("namespace", namespace),
		zap.String("deployment-name", deploymentName),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", ctxutil.DurationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", ctxutil.TimeLeftTillDeadline(ctx)),
		zap.Int32("target-available-replicas", targetAvailableReplicas),
	)
	sp.Restart()
	select {
	case <-stopc:
		sp.Stop()
		return nil, errors.New("initial wait aborted")
	case <-time.After(initialWait):
		sp.Stop()
	}

	retryWaitFunc := func() (done bool, err error) {
		select {
		case <-stopc:
			return true, errors.New("wait aborted")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dp, err = k8sClient.KubernetesClientSet().
			AppsV1().
			Deployments(namespace).
			Get(ctx, deploymentName, metav1.GetOptions{})
		cancel()
		if ret.queryFunc != nil {
			ret.queryFunc()
		}
		if err != nil {
			lg.Warn("failed to get Deployment", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
			return false, err
		}

		var dpCond appsv1.DeploymentCondition
		for _, cond := range dp.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				continue
			}
			dpCond = cond
			break
		}
		lg.Info("fetched Deployment",
			zap.Int32("desired-replicas", dp.Status.Replicas),
			zap.Int32("available-replicas", dp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dp.Status.ReadyReplicas),
			zap.String("condition-last-updated", dpCond.LastUpdateTime.String()),
			zap.String("condition-type", string(dpCond.Type)),
			zap.String("condition-status", string(dpCond.Status)),
			zap.String("condition-reason", dpCond.Reason),
			zap.String("condition-message", dpCond.Message),
		)
		if dpCond.Type == appsv1.DeploymentReplicaFailure {
			return true, fmt.Errorf("Deployment %q status %q", deploymentName, dpCond.Type)
		}
		if dp.Status.AvailableReplicas >= targetAvailableReplicas {
			if dpCond.Type == appsv1.DeploymentAvailable {
				return true, nil
			}
			lg.Warn("not all replicas available but more than target replicas; returning",
				zap.Int32("available", dp.Status.AvailableReplicas),
				zap.Int32("target", targetAvailableReplicas),
			)
			return true, nil
		}
		return false, nil
	}
	err = wait.PollImmediate(pollInterval, ctxutil.DurationTillDeadline(ctx), retryWaitFunc)
	return dp, err
}

// Op represents a Kubernetes client operation.
type Op struct {
	labelSelector   string
	fieldSelector   string
	queryFunc       func()
	podFunc         func(v1.Pod)
	forceDelete     bool
	forceDeleteFunc func()
}

// OpOption configures Kubernetes client operations.
type OpOption func(*Op)

// WithLabelSelector configures label selector for list operations.
func WithLabelSelector(s string) OpOption {
	return func(op *Op) { op.labelSelector = s }
}

// WithFieldSelector configures field selector for list operations.
func WithFieldSelector(s string) OpOption {
	return func(op *Op) { op.fieldSelector = s }
}

// WithQueryFunc configures query function to be called in retry func.
func WithQueryFunc(f func()) OpOption {
	return func(op *Op) { op.queryFunc = f }
}

// WithPodFunc configures function to be called for pod.
func WithPodFunc(f func(v1.Pod)) OpOption {
	return func(op *Op) { op.podFunc = f }
}

// WithForceDelete configures force delete.
// Useful for namespace deletion.
// ref. https://github.com/kubernetes/kubernetes/issues/60807
func WithForceDelete(forceDelete bool) OpOption {
	return func(op *Op) { op.forceDelete = forceDelete }
}

// WithForceDeleteFunc configures force delete.
// Useful for namespace deletion.
// ref. https://github.com/kubernetes/kubernetes/issues/60807
func WithForceDeleteFunc(forceDeleteFunc func()) OpOption {
	return func(op *Op) { op.forceDeleteFunc = forceDeleteFunc }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
