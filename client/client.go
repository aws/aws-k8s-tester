// Package client implements Kubernetes client utilities.
package client

import (
	"net"
	"time"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_util_net "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Reference
// https://pkg.go.dev/k8s.io/apimachinery/pkg/api/errors#pkg-overview

var (
	deleteGracePeriod = int64(0)
	deleteForeground  = meta_v1.DeletePropagationForeground
	deleteOption      = meta_v1.DeleteOptions{
		GracePeriodSeconds: &deleteGracePeriod,
		PropagationPolicy:  &deleteForeground,
	}
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
	if k8s_errors.IsInternalError(err) || k8s_errors.IsTimeout(err) || k8s_errors.IsServerTimeout(err) ||
		k8s_errors.IsTooManyRequests(err) || k8s_util_net.IsProbableEOF(err) || k8s_util_net.IsConnectionReset(err) ||
		// Retryable resource-quotas conflict errors may be returned in some cases, e.g. https://github.com/kubernetes/kubernetes/issues/67761
		isResourceQuotaConflictError(err) ||
		// Our client is using OAuth2 where 401 (unauthorized) can mean that our token has expired and we need to retry with a new one.
		k8s_errors.IsUnauthorized(err) {
		return true
	}
	// If the error sends the Retry-After header, we respect it as an explicit confirmation we should retry.
	if _, shouldRetry := k8s_errors.SuggestsClientDelay(err); shouldRetry {
		return true
	}
	return false
}

func isResourceQuotaConflictError(err error) bool {
	apiErr, ok := err.(k8s_errors.APIStatus)
	if !ok {
		return false
	}
	if apiErr.Status().Reason != meta_v1.StatusReasonConflict {
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
