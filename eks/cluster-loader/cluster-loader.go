// Package clusterloader implements cluster load tests.
// ref. https://github.com/kubernetes/perf-tests
package clusterloader

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	// Create installs Cluster Loader.
	Create() error
	// Delete deletes Cluster Loader.
	Delete() error
}

// TODO: use kubemark
// nodelease.NewController, kubemark.GetHollowKubeletConfig

func NewTester(cfg Config) (Tester, error) {
	return &tester{cfg: cfg, donec: make(chan struct{})}, nil
}

type tester struct {
	cfg   Config
	donec chan struct{}
}

func (ts *tester) Create() error {
	if ts.cfg.EKSConfig.AddOnClusterLoader.Created {
		ts.cfg.Logger.Info("skipping create AddOnClusterLoader")
		return nil
	}

	ts.cfg.Logger.Info("starting load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoader.Duration))
	ts.cfg.EKSConfig.AddOnClusterLoader.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoader.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnClusterLoader.CreateTookString = ts.cfg.EKSConfig.AddOnClusterLoader.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	deadline := time.Now().Add(ts.cfg.EKSConfig.AddOnClusterLoader.Duration)
	for i := 0; i < ts.cfg.EKSConfig.Clients; i++ {
		go listNodes(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listPods(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listServices(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listEndpoints(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listSecrets(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listConfigMaps(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listServiceAccounts(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listJobs(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
		go listCronJobs(ts.cfg.Logger, deadline, ts.cfg.Stopc, ts.donec, ts.cfg.K8SClient.KubernetesClientSet())
	}

	select {
	case <-ts.cfg.Stopc:
		ts.cfg.Logger.Warn("cluster loader aborted")
		close(ts.donec)
		return nil
	case <-time.After(ts.cfg.EKSConfig.AddOnClusterLoader.Duration):
		close(ts.donec)
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoader.Duration))

		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("cluster loader aborted")
			return nil
		case <-time.After(30 * time.Second):
		}
	}

	var err error
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
	if !ts.cfg.EKSConfig.AddOnClusterLoader.Created {
		ts.cfg.Logger.Info("skipping delete AddOnClusterLoader")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnClusterLoader.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnClusterLoader.DeleteTookString = ts.cfg.EKSConfig.AddOnClusterLoader.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoader.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func listNodes(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list nodes stopped")
			return
		case <-donec:
			lg.Info("list nodes done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list nodes failed", zap.Error(err))
		} else {
			if cnt%50 == 0 {
				lg.Info("listed nodes", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("nodes", len(rs.Items)))
			}
		}
	}
}

func listPods(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list pods stopped")
			return
		case <-donec:
			lg.Info("list pods done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			pods, err := cli.CoreV1().Pods(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list pods failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed pods", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("pods", len(pods.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list pods stopped")
				return
			case <-donec:
				lg.Info("list pods done")
				return
			default:
			}
		}
	}
}

func listServices(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list services stopped")
			return
		case <-donec:
			lg.Info("list services done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			es, err := cli.CoreV1().Services(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list services failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed services", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("services", len(es.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list services stopped")
				return
			case <-donec:
				lg.Info("list services done")
				return
			default:
			}
		}
	}
}

func listEndpoints(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list endpoints stopped")
			return
		case <-donec:
			lg.Info("list endpoints done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			es, err := cli.CoreV1().Endpoints(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list endpoints failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed endpoints", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("endpoints", len(es.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list endpoints stopped")
				return
			case <-donec:
				lg.Info("list endpoints done")
				return
			default:
			}
		}
	}
}

func listSecrets(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list secrets stopped")
			return
		case <-donec:
			lg.Info("list secrets done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			ss, err := cli.CoreV1().Secrets(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list secrets failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed secrets", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("secrets", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list secrets stopped")
				return
			case <-donec:
				lg.Info("list secrets done")
				return
			default:
			}
		}
	}
}

func listConfigMaps(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list configmaps stopped")
			return
		case <-donec:
			lg.Info("list configmaps done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			ss, err := cli.CoreV1().ConfigMaps(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list configmaps failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed configmaps", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("configmaps", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list configmaps stopped")
				return
			case <-donec:
				lg.Info("list configmaps done")
				return
			default:
			}
		}
	}
}

func listServiceAccounts(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list serviceaccounts stopped")
			return
		case <-donec:
			lg.Info("list serviceaccounts done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			ss, err := cli.CoreV1().ServiceAccounts(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list serviceaccounts failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed serviceaccounts", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("serviceaccounts", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list serviceaccounts stopped")
				return
			case <-donec:
				lg.Info("list serviceaccounts done")
				return
			default:
			}
		}
	}
}

func listJobs(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list jobs stopped")
			return
		case <-donec:
			lg.Info("list jobs done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			ss, err := cli.BatchV1().Jobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list jobs failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed jobs", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("jobs", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list jobs stopped")
				return
			case <-donec:
				lg.Info("list jobs done")
				return
			default:
			}
		}
	}
}

func listCronJobs(lg *zap.Logger, deadline time.Time, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	cnt := 0
	for {
		cnt++
		select {
		case <-stopc:
			lg.Warn("list cronjobs stopped")
			return
		case <-donec:
			lg.Info("list cronjobs done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ns, err := cli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list namespaces failed", zap.Error(err))
			continue
		}
		if cnt%50 == 0 {
			lg.Info("listed namespaces", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.Int("namespaces", len(ns.Items)))
		}

		for _, item := range ns.Items {
			nv := item.GetName()

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			ss, err := cli.BatchV1beta1().CronJobs(nv).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list cronjobs failed", zap.String("namespace", nv), zap.Error(err))
				continue
			}
			if cnt%50 == 0 {
				lg.Info("listed cronjobs", zap.String("time", humanize.Time(deadline)), zap.Int("iteration", cnt), zap.String("namespace", nv), zap.Int("cronjobs", len(ss.Items)))
			}

			select {
			case <-stopc:
				lg.Warn("list cronjobs stopped")
				return
			case <-donec:
				lg.Info("list jobs done")
				return
			default:
			}
		}
	}
}
