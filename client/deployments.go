package client

import (
	"context"
	"errors"
	"fmt"
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

// WaitForDeploymentAvailables waits till target replicas are ready in the Deployment.
func WaitForDeploymentAvailables(
	ctx context.Context,
	lg *zap.Logger,
	logWriter io.Writer,
	stopc chan struct{},
	c k8s_client.Interface,
	initialWait time.Duration,
	pollInterval time.Duration,
	namespace string,
	deploymentName string,
	targetAvailableReplicas int32,
	opts ...OpOption) (dp *apps_v1.Deployment, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	if pollInterval == 0 {
		pollInterval = DefaultNamespacePollInterval
	}

	sp := newSpinner(logWriter, "Waiting for Deployment availables "+deploymentName)
	lg.Info("waiting Deployment availables",
		zap.String("namespace", namespace),
		zap.String("deployment-name", deploymentName),
		zap.String("initial-wait", initialWait.String()),
		zap.String("poll-interval", pollInterval.String()),
		zap.String("ctx-duration-left", durationTillDeadline(ctx).String()),
		zap.String("ctx-time-left", timeLeftTillDeadline(ctx)),
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
		dp, err = c.AppsV1().
			Deployments(namespace).
			Get(ctx, deploymentName, meta_v1.GetOptions{})
		cancel()
		if ret.queryFunc != nil {
			ret.queryFunc()
		}
		if err != nil {
			lg.Warn("failed to get Deployment", zap.Bool("retriable-error", IsRetryableAPIError(err)), zap.Error(err))
			return false, err
		}

		var dpCond apps_v1.DeploymentCondition
		for _, cond := range dp.Status.Conditions {
			if cond.Status != core_v1.ConditionTrue {
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
			zap.Int32("target-available-replicas", targetAvailableReplicas),
			zap.String("condition-last-updated", dpCond.LastUpdateTime.String()),
			zap.String("condition-type", string(dpCond.Type)),
			zap.String("condition-status", string(dpCond.Status)),
			zap.String("condition-reason", dpCond.Reason),
			zap.String("condition-message", dpCond.Message),
		)
		if dpCond.Type == apps_v1.DeploymentReplicaFailure {
			return true, fmt.Errorf("deployment %q status %q", deploymentName, dpCond.Type)
		}
		if dp.Status.AvailableReplicas >= targetAvailableReplicas {
			if dpCond.Type == apps_v1.DeploymentAvailable {
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
	err = wait.PollImmediate(pollInterval, durationTillDeadline(ctx), retryWaitFunc)
	return dp, err
}

// DeleteDeployment deletes namespace with given name.
func DeleteDeployment(lg *zap.Logger, c k8s_client.Interface, namespace string, deploymentName string) error {
	deleteFunc := func() error {
		lg.Info("deleting Deployment", zap.String("namespace", namespace), zap.String("name", deploymentName))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			AppsV1().
			Deployments(namespace).
			Delete(
				ctx,
				deploymentName,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted Deployment", zap.String("namespace", namespace), zap.String("name", deploymentName))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("Deployment already deleted", zap.String("namespace", namespace), zap.String("name", deploymentName), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete Deployment", zap.String("namespace", namespace), zap.String("name", deploymentName), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}
