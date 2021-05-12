package client

import (
	"context"
	"errors"
	"io"
	"time"

	"go.uber.org/zap"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s_client "k8s.io/client-go/kubernetes"
)

// DeleteDaemonSet deletes namespace with given name.
func DeleteDaemonSet(lg *zap.Logger, c k8s_client.Interface, namespace string, Name string) error {
	deleteFunc := func() error {
		lg.Info("deleting DaemonSet", zap.String("namespace", namespace), zap.String("name", Name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			AppsV1().
			DaemonSets(namespace).
			Delete(
				ctx,
				Name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted DaemonSets", zap.String("namespace", namespace), zap.String("name", Name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("DaemonSets already deleted", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete DaemonSets", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// WaitForDaemonSetCompletes waits till target replicas are ready in the Deployment.
func WaitForDaemonSetCompletes(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	c k8s_client.Interface,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	daemonsetName string,
	opts ...OpOption) (dp *apps_v1.DaemonSet, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := newSpinner(logWriter, "Waiting for Deployment completes "+daemonsetName)
	lg.Info("waiting DaemonSets completes",
		zap.String("namespace", namespace),
		zap.String("job-name", daemonsetName),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", durationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", timeLeftTillDeadline(ctx)),
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
		dp, err = c.AppsV1().
			DaemonSets(namespace).
			Get(ctx, daemonsetName, meta_v1.GetOptions{})
		cancel()
		if ret.queryFunc != nil {
			ret.queryFunc()
		}
		if err != nil {
			lg.Warn("failed to get DaemonSet", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
			return false, err
		}

		var dpCond apps_v1.DaemonSetCondition
		for _, cond := range dp.Status.Conditions {
			if cond.Status != core_v1.ConditionTrue {
				continue
			}
			dpCond = cond
			break
		}
		lg.Info("fetched DaemonSet",
			zap.Int32("current-number-scheduled", dp.Status.CurrentNumberScheduled),
			zap.Int32("number-misscheduled", dp.Status.NumberMisscheduled),
			zap.Int32("desired-number-scheduled", dp.Status.DesiredNumberScheduled),
			zap.Int32("number-ready", dp.Status.NumberReady),
			zap.String("condition-type", string(dpCond.Type)),
			zap.String("condition-status", string(dpCond.Status)),
			zap.String("condition-reason", dpCond.Reason),
			zap.String("condition-message", dpCond.Message),
		)
		if dp.Status.DesiredNumberScheduled >= dp.Status.NumberReady {
			lg.Warn("pods available; returning",
				zap.Int32("available", dp.Status.NumberReady),
				zap.Int32("target", dp.Status.DesiredNumberScheduled),
			)
			return true, nil
		}
		return false, nil
	}
	err = wait.PollImmediate(pollInterval, durationTillDeadline(ctx), retryWaitFunc)
	return dp, err
}
