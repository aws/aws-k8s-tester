package client

import (
	"context"
	"time"

	"go.uber.org/zap"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	k8s_client "k8s.io/client-go/kubernetes"
)

// DeleteService deletes namespace with given name.
func DeleteServiceAccount(lg *zap.Logger, c k8s_client.Interface, namespace string, name string) error {
	deleteFunc := func() error {
		lg.Info("deleting ServiceAccount", zap.String("namespace", namespace), zap.String("name", name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			CoreV1().
			ServiceAccounts(namespace).
			Delete(
				ctx,
				name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted ServiceAccount", zap.String("namespace", namespace), zap.String("name", name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("ServiceAccount already deleted", zap.String("namespace", namespace), zap.String("name", name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete ServiceAccount", zap.String("namespace", namespace), zap.String("name", name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}
