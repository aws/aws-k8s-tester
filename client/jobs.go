package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beta1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s_client "k8s.io/client-go/kubernetes"
)

// WaitForJobCompletes waits for all Job completion,
// by counting the number of pods in the namespace.
func WaitForJobCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	c k8s_client.Interface,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (job *batch_v1.Job, pods []core_v1.Pod, err error) {
	job, _, pods, err = waitForJobCompletes(false, ctx, lg, logWriter, stopc, c, initialWait, pollInterval, namespace, jobName, targetCompletes, opts...)
	return job, pods, err
}

// WaitForCronJobCompletes waits for all CronJob completion,
// by counting the number of pods in the namespace.
func WaitForCronJobCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	c k8s_client.Interface,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (cronJob *batch_v1beta1.CronJob, pods []core_v1.Pod, err error) {
	_, cronJob, pods, err = waitForJobCompletes(true, ctx, lg, logWriter, stopc, c, initialWait, pollInterval, namespace, jobName, targetCompletes, opts...)
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
	c k8s_client.Interface,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	jobName string,
	targetCompletes int,
	opts ...OpOption) (job *batch_v1.Job, cronJob *batch_v1beta1.CronJob, pods []core_v1.Pod, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := newSpinner(logWriter, "Waiting for Job completes "+jobName)
	lg.Info("waiting Job completes",
		zap.String("namespace", namespace),
		zap.String("job-name", jobName),
		zap.Bool("cron-job", isCronJob),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", durationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", timeLeftTillDeadline(ctx)),
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
		pods, err = ListPods(lg, c, namespace, 3000, 3*time.Second)
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
			if pod.Status.Phase == core_v1.PodSucceeded {
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
				zap.String("ctx-time-left", timeLeftTillDeadline(ctx)),
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
			zap.String("ctx-time-left", timeLeftTillDeadline(ctx)),
		)

		switch isCronJob {
		case false:
			lg.Info("checking Job object", zap.String("namespace", namespace))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			job, err = c.
				BatchV1().
				Jobs(namespace).
				Get(ctx, jobName, meta_v1.GetOptions{})
			cancel()
			if err != nil {
				lg.Warn("failed to check Job", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
				return false, err
			}
			for _, cond := range job.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type == batch_v1.JobFailed {
					lg.Warn("job failed", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
					return true, fmt.Errorf("Job %q status %q", jobName, cond.Type)
				}
				if cond.Type == batch_v1.JobComplete {
					lg.Info("job complete", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
					return true, nil
				}
				lg.Warn("job not complete", zap.String("condition-type", fmt.Sprintf("%s", cond.Type)))
			}

		case true:
			lg.Info("checking CronJob object", zap.String("namespace", namespace))
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			cronJob, err = c.
				BatchV1beta1().
				CronJobs(namespace).
				Get(ctx, jobName, meta_v1.GetOptions{})
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
	err = wait.PollImmediate(pollInterval, durationTillDeadline(ctx), retryWaitFunc)
	return job, cronJob, pods, err
}
