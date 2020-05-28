// Package local implements local cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
package local

import (
	"context"
	"errors"
	"strings"
	"time"

	hollow_nodes "github.com/aws/aws-k8s-tester/eks/hollow-nodes"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config defines hollow nodes configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines hollow nodes tester.
type Tester interface {
	// Create installs hollow nodes.
	Create() error
	// Delete deletes hollow nodes.
	Delete() error
}

func New(cfg Config) (Tester, error) {
	ts := &tester{cfg: cfg}
	var err error
	ts.ng, err = hollow_nodes.CreateNodeGroup(hollow_nodes.NodeGroupConfig{
		Logger:         ts.cfg.Logger,
		Stopc:          ts.cfg.Stopc,
		Client:         ts.cfg.K8SClient,
		Nodes:          ts.cfg.EKSConfig.AddOnHollowNodesLocal.Nodes,
		NodeNamePrefix: ts.cfg.EKSConfig.AddOnHollowNodesLocal.NodeNamePrefix,
		NodeLabels:     ts.cfg.EKSConfig.AddOnHollowNodesLocal.NodeLabels,
		MaxOpenFiles:   ts.cfg.EKSConfig.AddOnHollowNodesLocal.MaxOpenFiles,
		Remote:         false,
	})
	if err != nil {
		return nil, err
	}
	return ts, nil
}

type tester struct {
	cfg Config
	ng  hollow_nodes.NodeGroup
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnHollowNodesLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnHollowNodesLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting hollow nodes testing")
	ts.cfg.EKSConfig.AddOnHollowNodesLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnHollowNodesLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.ng.Start(); err != nil {
		return err
	}
	_, ts.cfg.EKSConfig.AddOnHollowNodesLocal.CreatedNodeNames, err = ts.ng.CheckNodes()
	if err != nil {
		return err
	}
	ts.cfg.EKSConfig.Sync()

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
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.AddOnHollowNodesLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnHollowNodesLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnHollowNodesLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := ts.deleteCreatedNodes(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnHollowNodesLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteCreatedNodes() error {
	var errs []string

	ts.cfg.Logger.Info("deleting node objects", zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodesLocal.CreatedNodeNames)))
	deleted := 0
	foreground := metav1.DeletePropagationForeground
	for i, nodeName := range ts.cfg.EKSConfig.AddOnHollowNodesLocal.CreatedNodeNames {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().Delete(
			ctx,
			nodeName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
		cancel()
		if err != nil && !api_errors.IsNotFound(err) {
			ts.cfg.Logger.Warn("failed to delete node", zap.Int("index", i), zap.String("name", nodeName), zap.Error(err))
			errs = append(errs, err.Error())
		} else {
			ts.cfg.Logger.Info("deleted node", zap.Int("index", i), zap.String("name", nodeName))
			deleted++
		}
	}
	ts.cfg.Logger.Info("deleted node objects", zap.Int("deleted", deleted), zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodesLocal.CreatedNodeNames)))

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}
