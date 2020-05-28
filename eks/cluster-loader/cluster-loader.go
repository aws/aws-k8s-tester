// Package clusterloader implements cluster loader.
// ref. https://github.com/kubernetes/perf-tests/tree/master/clusterloader2
package clusterloader

import (
	"sync"
	"time"

	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
)

// Config configures cluster loader.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	Client        k8s_client.EKS
	ClientTimeout time.Duration

	Namespace string
}

// Loader defines cluster loader operations.
type Loader interface {
	Start()
	Stop()
	GetResults()
}

type loader struct {
	cfg            Config
	donec          chan struct{}
	donecCloseOnce *sync.Once
}

func New(cfg Config) Loader {
	return &loader{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

func (ld *loader) Start() {
	ld.cfg.Logger.Info("starting cluster loader")
	ld.cfg.Logger.Info("completed cluster loader")
}

func (ld *loader) Stop() {
	ld.cfg.Logger.Info("stopping and waiting for cluster loader")
	ld.donecCloseOnce.Do(func() {
		close(ld.donec)
	})
	ld.cfg.Logger.Info("stopped and waited for cluster loader")
}

func (ld *loader) GetResults() {

}
