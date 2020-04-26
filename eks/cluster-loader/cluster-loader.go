// Package clusterloader implements cluster load tests.
// ref. https://github.com/kubernetes/perf-tests
package clusterloader

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
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
	return &tester{cfg: cfg, cancelc: make(chan struct{})}, nil
}

type tester struct {
	cfg     Config
	cancelc chan struct{}
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

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnClusterLoader.Namespace); err != nil {
		return err
	}

	for i := 0; i < ts.cfg.EKSConfig.Clients; i++ {
		go listNodes(ts.cfg.Logger, ts.cfg.Stopc, ts.cancelc, ts.cfg.K8SClient.KubernetesClientSet())
		go listPods(ts.cfg.Logger, ts.cfg.Stopc, ts.cancelc, ts.cfg.K8SClient.KubernetesClientSet())
		go writeEvents(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnClusterLoader.Namespace, ts.cfg.Stopc, ts.cancelc, ts.cfg.K8SClient.KubernetesClientSet())
		go listEvents(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnClusterLoader.Namespace, ts.cfg.Stopc, ts.cancelc, ts.cfg.K8SClient.KubernetesClientSet())
	}
	select {
	case <-ts.cfg.Stopc:
	case <-time.After(ts.cfg.EKSConfig.AddOnClusterLoader.Duration):
		close(ts.cancelc)
		ts.cfg.Logger.Info("completing load testing", zap.Duration("duration", ts.cfg.EKSConfig.AddOnClusterLoader.Duration))
	}

	return ts.cfg.EKSConfig.Sync()
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

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnClusterLoader.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Cluster Loader namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnClusterLoader.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func listNodes(lg *zap.Logger, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	for {
		select {
		case <-stopc:
			lg.Warn("list node stopped")
			return
		case <-donec:
			lg.Info("list node done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list nodes failed", zap.Error(err))
		} else {
			lg.Info("listed nodes", zap.Int("nodes", len(rs.Items)))
		}
	}
}

func listPods(lg *zap.Logger, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	for {
		select {
		case <-stopc:
			lg.Warn("list node stopped")
			return
		case <-donec:
			lg.Info("list node done")
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
		lg.Info("listed namespaces", zap.Int("namespaces", len(ns.Items)))

		for _, item := range ns.Items {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			pods, err := cli.CoreV1().Pods(item.GetName()).List(ctx, metav1.ListOptions{})
			cancel()
			if err != nil {
				lg.Warn("list pods failed", zap.String("namespace", item.GetName()), zap.Error(err))
				continue
			}
			lg.Info("listed pods", zap.String("namespace", item.GetName()), zap.Int("pods", len(pods.Items)))
		}
	}
}

func writeEvents(lg *zap.Logger, namespace string, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	for {
		select {
		case <-stopc:
			lg.Warn("list node stopped")
			return
		case <-donec:
			lg.Info("list node done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := cli.CoreV1().Events(namespace).Create(
			ctx,
			&v1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test-event-%s-%x", namespace, randNum()),
				},
				Reason:  "testing events",
				Message: fmt.Sprintf("%x", randNum()),
			},
			metav1.CreateOptions{},
		)
		cancel()
		if err != nil {
			lg.Warn("create event failed", zap.Error(err))
		} else {
			lg.Info("created events")
		}
	}
}

func listEvents(lg *zap.Logger, namespace string, stopc chan struct{}, donec chan struct{}, cli *kubernetes.Clientset) {
	for {
		select {
		case <-stopc:
			lg.Warn("list node stopped")
			return
		case <-donec:
			lg.Info("list node done")
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		evs, err := cli.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			lg.Warn("list event failed", zap.Error(err))
		} else {
			lg.Info("listed events", zap.Int("events", len(evs.Items)))
		}
	}
}

func randNum() int64 {
	ts := time.Now().UnixNano()
	sign := int64(-1)
	if ts%2 == 0 {
		sign = 1
	}
	return time.Now().UnixNano() + sign*int64(rand.Intn(3000))
}
