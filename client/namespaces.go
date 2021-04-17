package client

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	k8s_client "k8s.io/client-go/kubernetes"
)

// ListNamespaces returns list of existing namespace names.
func ListNamespaces(c k8s_client.Interface) ([]core_v1.Namespace, error) {
	var namespaces []core_v1.Namespace
	listFunc := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		namespacesList, err := c.CoreV1().Namespaces().List(ctx, meta_v1.ListOptions{})
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

// CreateNamespace creates a single namespace with given name.
func CreateNamespace(lg *zap.Logger, c k8s_client.Interface, namespace string) error {
	createFunc := func() error {
		lg.Info("creating namespace", zap.String("namespace", namespace))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := c.CoreV1().Namespaces().Create(ctx, &core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: namespace}}, meta_v1.CreateOptions{})
		cancel()
		if err == nil {
			lg.Info("created namespace", zap.String("namespace", namespace))
			return nil
		}
		if k8s_errors.IsAlreadyExists(err) {
			lg.Info("namespace already exists", zap.String("namespace", namespace), zap.Error(err))
			return nil
		}
		lg.Warn("failed to create namespace", zap.String("namespace", namespace), zap.Error(err))
		return err
	}
	return RetryWithExponentialBackOff(RetryFunction(createFunc, Allow(k8s_errors.IsAlreadyExists)))
}

// DeleteNamespaceAndWait deletes namespace with given name and waits for its deletion.
// Default interval is 5-second and default timeout is 10-min.
func DeleteNamespaceAndWait(
	lg *zap.Logger,
	c k8s_client.Interface,
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
func deleteNamespace(lg *zap.Logger, c k8s_client.Interface, namespace string) error {
	foreground, zero := meta_v1.DeletePropagationForeground, int64(0)
	deleteFunc := func() error {
		lg.Info("deleting namespace", zap.String("namespace", namespace))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.CoreV1().Namespaces().Delete(
			ctx,
			namespace,
			meta_v1.DeleteOptions{
				GracePeriodSeconds: &zero,
				PropagationPolicy:  &foreground,
			},
		)
		cancel()
		if err == nil {
			lg.Info("deleted namespace", zap.String("namespace", namespace))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("namespace already deleted", zap.String("namespace", namespace), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete namespace", zap.String("namespace", namespace), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

func waitForDeleteNamespace(lg *zap.Logger, cli k8s_client.Interface, namespace string, pollInterval time.Duration, timeout time.Duration, opts ...OpOption) error {
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
		var ns *core_v1.Namespace
		ns, err = cli.CoreV1().Namespaces().Get(ctx, namespace, meta_v1.GetOptions{})
		cancel()
		if err != nil {
			if k8s_errors.IsNotFound(err) {
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
			_, err = cli.CoreV1().Namespaces().Update(ctx, ns, meta_v1.UpdateOptions{})
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
