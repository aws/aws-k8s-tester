// Package local implements Secrets plugin.
package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/secrets"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
)

// Config defines secrets local tester configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines secrets local tester.
type Tester interface {
	// Create installs secrets local tester.
	Create() error
	// Delete deletes secrets local tester.
	Delete() error
}

func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnSecretsLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnSecretsLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting secrets local tester")
	ts.cfg.EKSConfig.AddOnSecretsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnSecretsLocal.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnSecretsLocal.CreateTookString = ts.cfg.EKSConfig.AddOnSecretsLocal.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
	); err != nil {
		return err
	}

	loader := secrets.New(secrets.Config{
		Logger:        ts.cfg.Logger,
		Stopc:         ts.cfg.Stopc,
		Client:        ts.cfg.K8SClient,
		ClientTimeout: ts.cfg.EKSConfig.ClientTimeout,
		Namespace:     ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
		NamePrefix:    ts.cfg.EKSConfig.AddOnSecretsLocal.NamePrefix,
		Objects:       ts.cfg.EKSConfig.AddOnSecretsLocal.Objects,
		ObjectSize:    ts.cfg.EKSConfig.AddOnSecretsLocal.ObjectSize,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing secrets local tester")
	ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWrites, ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReads, err = loader.GetMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
	} else {
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWritesJSONPath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWrites.JSON()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWritesTablePath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWrites.Table()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		fmt.Printf("\n\nAddOnSecretsLocal.RequestsSummaryWrites:\n%s\n", ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryWrites.Table())
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReadsJSONPath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReads.JSON()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReadsTablePath, []byte(ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReads.Table()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		fmt.Printf("\n\nAddOnSecretsLocal.RequestsSummaryReads:\n%s\n", ts.cfg.EKSConfig.AddOnSecretsLocal.RequestsSummaryReads.Table())
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
		ts.cfg.Logger.Info("health check success after secrets local tester")
	} else {
		ts.cfg.Logger.Warn("health check failed after secrets local tester", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnSecretsLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnSecretsLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnSecretsLocal.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnSecretsLocal.DeleteTookString = ts.cfg.EKSConfig.AddOnSecretsLocal.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if err := k8s_client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnSecretsLocal.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout,
	); err != nil {
		return fmt.Errorf("failed to delete secrets local tester namespace (%v)", err)
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnSecretsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}
