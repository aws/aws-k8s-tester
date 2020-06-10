// Package versionupgrade implements EKS cluster version upgrade tester.
package versionupgrade

import (
	"reflect"
	"time"

	eks_tester "github.com/aws/aws-k8s-tester/eks/tester"
	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/timeutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"go.uber.org/zap"
)

// Config defines version upgrade configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS
	EKSAPI    eksiface.EKSAPI
}

// New creates a new Job tester.
func New(cfg Config) eks_tester.Tester {
	cfg.Logger.Info("creating tester", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return &tester{cfg: cfg}
}

type tester struct {
	cfg Config
}

func (ts *tester) Create() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Create", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()
	defer func() {
		createEnd := time.Now()
		ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.TimeFrameCreate = timeutil.NewTimeFrame(createStart, createEnd)
		ts.cfg.EKSConfig.Sync()
	}()

	ts.cfg.Logger.Info("starting cluster version upgrade",
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	out, err := ts.cfg.EKSAPI.UpdateClusterVersion(&eks.UpdateClusterVersionInput{
		Name:    aws.String(ts.cfg.EKSConfig.Name),
		Version: aws.String(ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	})
	if err != nil {
		return err
	}
	reqID := ""
	if out.Update != nil {
		reqID = aws.StringValue(out.Update.Id)
	}
	ts.cfg.Logger.Info("sent upgrade cluster request", zap.String("request-id", reqID))

	// TODO: polling update
	// TODO: polling cluster
	// TODO: health check

	ts.cfg.Logger.Info("completed cluster version upgrade",
		zap.String("from", ts.cfg.EKSConfig.Parameters.Version),
		zap.String("to", ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Version),
	)
	return nil
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.Delete", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) AggregateResults() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnClusterVersionUpgrade() {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnClusterVersionUpgrade.Created {
		ts.cfg.Logger.Info("skipping tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
		return nil
	}

	ts.cfg.Logger.Info("starting tester.AggregateResults", zap.String("tester", reflect.TypeOf(tester{}).PkgPath()))
	return nil
}
