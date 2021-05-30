package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
)

// DeleteService deletes namespace with given name.
func DeleteService(lg *zap.Logger, c k8s_client.Interface, namespace string, svcName string) error {
	deleteFunc := func() error {
		lg.Info("deleting Service", zap.String("namespace", namespace), zap.String("name", svcName))
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.
			CoreV1().
			Services(namespace).
			Delete(
				ctx,
				svcName,
				deleteOption,
			)
		cancel()
		if err == nil {
			lg.Info("deleted Service", zap.String("namespace", namespace), zap.String("name", svcName))
			return nil
		}
		if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
			lg.Info("Service already deleted", zap.String("namespace", namespace), zap.String("name", svcName), zap.Error(err))
			return nil
		}
		lg.Warn("failed to delete Service", zap.String("namespace", namespace), zap.String("name", svcName), zap.Error(err))
		return err
	}
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
	return RetryWithExponentialBackOff(RetryFunction(deleteFunc, Allow(k8s_errors.IsNotFound)))
}

// WaitForServiceIngressHostname waits for Service's Ingress Hostname to be updated
// and returns the ELB ARN.
func WaitForServiceIngressHostname(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	svcName string,
	stopc chan struct{},
	waitDur time.Duration,
	accountID string,
	region string,
	opts ...OpOption) (hostName string, elbARN string, elbName string, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	lg.Info("waiting for service",
		zap.String("namespace", namespace),
		zap.String("service-name", svcName),
	)

	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-stopc:
			return "", "", "", errors.New("wait for service aborted")
		case <-time.After(5 * time.Second):
		}

		if ret.queryFunc != nil {
			ret.queryFunc()
		}

		lg.Info("querying Service for ingress endpoint",
			zap.String("namespace", namespace),
			zap.String("service-name", svcName),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		so, err := c.
			CoreV1().
			Services(namespace).
			Get(ctx, svcName, meta_v1.GetOptions{})
		cancel()
		if err != nil {
			lg.Warn("failed to get Service; retrying", zap.Error(err))
			if k8s_errors.IsNotFound(err) {
				time.Sleep(20 * time.Second)
			}
			time.Sleep(5 * time.Second)
			continue
		}

		lg.Info(
			"Service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			lg.Info(
				"Service has been linked to LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}

		if hostName != "" {
			lg.Info("found LoadBalancer host name", zap.String("host-name", hostName))
			break
		}
	}

	if hostName == "" {
		return "", "", "", errors.New("failed to find LoadBalancer host name")
	}

	// TODO: find better way to find out the NLB/ELB name
	elbName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	elbARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		region,
		accountID,
		ss,
	)
	lg.Info("found LoadBalancer ELB ARN", zap.String("elb-arn", elbARN))

	return hostName, elbARN, elbName, nil
}

// FindServiceIngressHostname finds Service's Ingress Hostname to be updated
// and returns the ELB ARN.
func FindServiceIngressHostname(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	svcName string,
	stopc chan struct{},
	waitDur time.Duration,
	accountID string,
	region string,
	opts ...OpOption) (hostName string, elbARN string, elbName string, exists bool, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	lg.Info("finding service ingress host name",
		zap.String("namespace", namespace),
		zap.String("service-name", svcName),
	)

	retryStart := time.Now()
	for time.Since(retryStart) < waitDur {
		select {
		case <-stopc:
			return "", "", "", true, errors.New("wait for service aborted")
		case <-time.After(5 * time.Second):
		}

		if ret.queryFunc != nil {
			ret.queryFunc()
		}

		lg.Info("querying Service for ingress endpoint",
			zap.String("namespace", namespace),
			zap.String("service-name", svcName),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		so, err := c.
			CoreV1().
			Services(namespace).
			Get(ctx, svcName, meta_v1.GetOptions{})
		cancel()
		if err != nil {
			lg.Warn("failed to get Service; retrying", zap.Error(err))
			if k8s_errors.IsNotFound(err) {
				return "", "", "", false, nil
			}
			time.Sleep(5 * time.Second)
			continue
		}

		lg.Info(
			"Service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			lg.Info(
				"Service has been linked to LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}

		if hostName != "" {
			lg.Info("found LoadBalancer host name", zap.String("host-name", hostName))
			break
		}
	}

	if hostName == "" {
		return "", "", "", true, errors.New("failed to find LoadBalancer host name")
	}

	// TODO: find better way to find out the NLB/ELB name
	elbName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	elbARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		region,
		accountID,
		ss,
	)
	lg.Info("found LoadBalancer ELB ARN", zap.String("elb-arn", elbARN))

	return hostName, elbARN, elbName, true, nil
}
