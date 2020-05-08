// Package local implements cluster local load tests.
// ref. https://github.com/kubernetes/perf-tests
package local

import (
	"context"
	"errors"
	"strings"
	"time"

	cluster_loader "github.com/aws/aws-k8s-tester/eks/cluster-loader"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config defines Cluster Loader configuration.
// ref. https://github.com/kubernetes/perf-tests
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines Cluster Loader tester.
// ref. https://github.com/kubernetes/perf-tests
type Tester interface {
	// Create installs cluster Loader.
	Create() error
	// Delete deletes cluster Loader.
	Delete() error
}

// TODO: use kubemark
// nodelease.NewController, kubemark.GetHollowKubeletConfig

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnClusterLoaderLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Duration))
	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CreateTookString = ts.cfg.EKSConfig.AddOnClusterLoaderLocal.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	nss, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	cancel()
	if err != nil {
		ts.cfg.Logger.Warn("list namespaces failed", zap.Error(err))
		return err
	}
	ns := make([]string, 0, len(nss.Items))
	for _, nv := range nss.Items {
		ns = append(ns, nv.GetName())
	}

	loader := cluster_loader.New(cluster_loader.Config{
		Logger:     ts.cfg.Logger,
		Client:     ts.cfg.K8SClient,
		Groups:     ts.cfg.EKSConfig.Clients,
		Stopc:      ts.cfg.Stopc,
		Deadline:   time.Now().Add(ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Duration),
		Timeout:    20 * time.Second,
		Namespaces: ns,
	})
	loader.Start()

	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("cluster loader aborted")
		success, failure, hs, err := loader.GetMetrics()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
		} else {
			ts.cfg.EKSConfig.AddOnClusterLoaderLocal.RequestsSummary = eksconfig.RequestsSummary{
				SuccessTotal:     success,
				FailureTotal:     failure,
				LatencyHistogram: hs,
			}
			ts.cfg.EKSConfig.Sync()
		}
		loader.Stop()
		return nil

	case <-time.After(ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Duration):
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Duration))
		success, failure, hs, err := loader.GetMetrics()
		if err != nil {
			ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
		} else {
			ts.cfg.EKSConfig.AddOnClusterLoaderLocal.RequestsSummary = eksconfig.RequestsSummary{
				SuccessTotal:     success,
				FailureTotal:     failure,
				LatencyHistogram: hs,
			}
			ts.cfg.EKSConfig.Sync()
		}
		loader.Stop()

		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader aborted")
			return nil
		case <-time.After(30 * time.Second):
		}
	}

	waitDur, retryStart := 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("health check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}
		err = ts.cfg.K8SClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
	}
	ts.cfg.EKSConfig.Sync()
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnClusterLoaderLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnClusterLoaderLocal.DeleteTookString = ts.cfg.EKSConfig.AddOnClusterLoaderLocal.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoaderLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}
