package client

import (
	"context"
	"time"

	"go.uber.org/zap"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	k8s_client "k8s.io/client-go/kubernetes"
)

// DeleteRBACClusterRole deletes ClusterRole with given name.
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func DeleteRBACClusterRole(lg *zap.Logger, c k8s_client.Interface, name string) error {
	deleteFunc := func() error {
		lg.Info("deleting RBACClusterRole", zap.String("name", name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			RbacV1().
			ClusterRoles().
			Delete(
				ctx,
				name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted RBACClusterRole", zap.String("name", name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("RBACClusterRole already deleted", zap.String("name", name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete RBACClusterRole", zap.String("name", name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// DeleteRBACClusterRole deletes ClusterRole with given name.
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func DeleteRBACClusterRoleBinding(lg *zap.Logger, c k8s_client.Interface, name string) error {
	deleteFunc := func() error {
		lg.Info("deleting RBACClusterRoleBinding", zap.String("name", name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			RbacV1().
			ClusterRoleBindings().
			Delete(
				ctx,
				name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted RBACClusterRoleBinding", zap.String("name", name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("RBACClusterRoleBinding already deleted", zap.String("name", name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete RBACClusterRoleBinding", zap.String("name", name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// DeleteRBACRole deletes Role with given name.
// ref. https://github.com/kubernetes/client-go/tree/master/examples/in-cluster-client-configuration
// ref. https://kubernetes.io/docs/reference/access-authn-authz/rbac/
func DeleteRBACRole(lg *zap.Logger, c k8s_client.Interface, namespace string, name string) error {
	deleteFunc := func() error {
		lg.Info("deleting RBACRole", zap.String("name", name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			RbacV1().
			Roles(namespace).
			Delete(
				ctx,
				name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted RBACRole", zap.String("name", name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("RBACRole already deleted", zap.String("name", name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete RBACRole", zap.String("name", name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// DeleteRBACRoleBinding deletes RoleBinding with given name.
func DeleteRBACRoleBinding(lg *zap.Logger, c k8s_client.Interface, namespace string, name string) error {
	deleteFunc := func() error {
		lg.Info("deleting RBACRoleBinding", zap.String("name", name))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			RbacV1().
			RoleBindings(namespace).
			Delete(
				ctx,
				name,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted RBACRoleBinding", zap.String("name", name))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("RBACRoleBinding already deleted", zap.String("name", name), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete RBACRoleBinding", zap.String("name", name), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}
