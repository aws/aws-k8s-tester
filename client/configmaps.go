package client

import (
	"context"
	"time"

	"go.uber.org/zap"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	k8s_client "k8s.io/client-go/kubernetes"
)

// DeleteService deletes namespace with given name.
func DeleteConfigmap(lg *zap.Logger, c k8s_client.Interface, namespace string, Name string) error {
	deleteFunc := func() error {
		lg.Info("deleting Configmap", zap.String("namespace", namespace), zap.String("name", Name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			CoreV1().
			ConfigMaps(namespace).
			Delete(
				ctx,
				Name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted Configmap", zap.String("namespace", namespace), zap.String("name", Name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("Configmap already deleted", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete Configmap", zap.String("namespace", namespace), zap.String("name", Name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}
