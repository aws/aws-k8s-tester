// Package local implements tester for CertificateSigningRequest.
package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eks/csrs"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"go.uber.org/zap"
)

// Config defines csrs local tester configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
}

// Tester defines csrs local tester tester.
type Tester interface {
	// Create installs csrs local tester.
	Create() error
	// Delete deletes csrs local tester.
	Delete() error
}

func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnCSRsLocal.Created {
		ts.cfg.Logger.Info("skipping create AddOnCSRsLocal")
		return nil
	}

	ts.cfg.Logger.Info("starting csrs local tester")
	ts.cfg.EKSConfig.AddOnCSRsLocal.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsLocal.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	loader := csrs.New(csrs.Config{
		Logger:                      ts.cfg.Logger,
		Stopc:                       ts.cfg.Stopc,
		Client:                      ts.cfg.K8SClient,
		ClientTimeout:               ts.cfg.EKSConfig.ClientTimeout,
		Objects:                     ts.cfg.EKSConfig.AddOnCSRsLocal.Objects,
		InitialRequestConditionType: ts.cfg.EKSConfig.AddOnCSRsLocal.InitialRequestConditionType,
		WritesJSONPath:              ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesJSONPath,
	})
	loader.Start()
	loader.Stop()

	ts.cfg.Logger.Info("completing csrs local tester")
	ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummary, err = loader.CollectMetrics()
	ts.cfg.EKSConfig.Sync()
	if err != nil {
		ts.cfg.Logger.Warn("failed to get metrics", zap.Error(err))
	} else {
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummaryJSONPath, []byte(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummary.JSON()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		err = ioutil.WriteFile(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummaryTablePath, []byte(ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummary.Table()), 0600)
		if err != nil {
			ts.cfg.Logger.Warn("failed to write file", zap.Error(err))
			return err
		}
		fmt.Printf("\n\nAddOnCSRsLocal.RequestsWritesSummary:\n%s\n", ts.cfg.EKSConfig.AddOnCSRsLocal.RequestsWritesSummary.Table())
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
		ts.cfg.Logger.Info("health check success after csrs local tester")
	} else {
		ts.cfg.Logger.Warn("health check failed after csrs local tester", zap.Error(err))
	}
	return err
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnCSRsLocal.Created {
		ts.cfg.Logger.Info("skipping delete AddOnCSRsLocal")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		deleteEnd := time.Now()
		ts.cfg.EKSConfig.AddOnCSRsLocal.TimeFrameDelete = timeutil.NewTimeFrame(deleteStart, deleteEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnCSRsLocal.Created = false
	return ts.cfg.EKSConfig.Sync()
}
