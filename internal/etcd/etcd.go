// Package etcd implements etcd test operations.
package etcd

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/etcdtester"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

type embedded struct {
	stopc chan struct{}

	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *etcdconfig.Config
}

// NewTester creates a new embedded etcd tester.
func NewTester(cfg *etcdconfig.Config) (etcdtester.Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}

	return &embedded{
		stopc: make(chan struct{}),
		lg:    lg,
		cfg:   cfg,
	}, nil
}

func (md *embedded) Deploy() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now().UTC()

	// expect the following in Plugins
	// "update-amazon-linux-2"
	// "install-etcd-3.1.12"
	md.lg.Info(
		"deploying EC2",
		zap.Strings("plugins", md.cfg.EC2.Plugins),
	)
	var ec2Deployer ec2.Deployer
	ec2Deployer, err = ec2.NewDeployer(md.cfg.EC2)
	if err != nil {
		return err
	}
	if err = ec2Deployer.Create(); err != nil {
		return err
	}
	md.cfg.Sync()
	md.lg.Info(
		"deployed EC2",
		zap.Strings("plugins", md.cfg.EC2.Plugins),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if md.cfg.LogDebug {
		fmt.Println(ec2Deployer.GenerateSSHCommands())
	}

	tc := *md.cfg.Cluster
	for _, iv := range md.cfg.EC2.Instances {
		ev := tc
		ev.TopLevel = false
		ev.Name = iv.InstanceID
		ev.DataDir = fmt.Sprintf("/home/%s/etcd.data", md.cfg.EC2.UserName)
		ev.ListenClientURLs = fmt.Sprintf("http://%s:2379", iv.PrivateIP)
		ev.AdvertiseClientURLs = fmt.Sprintf("http://%s:2379", iv.PublicIP)
		ev.ListenPeerURLs = fmt.Sprintf("http://%s:2380", iv.PrivateIP)
		ev.AdvertisePeerURLs = fmt.Sprintf("http://%s:2380", iv.PublicIP)
		ev.InitialCluster = ""
		ev.InitialClusterState = "new"
		md.cfg.ClusterState[iv.InstanceID] = ev
	}

	initialCluster := ""
	for k, v := range md.cfg.ClusterState {
		initialCluster += fmt.Sprintf(",%s=%s", k, v.AdvertisePeerURLs)
	}
	initialCluster = initialCluster[1:]

	for id, v := range md.cfg.ClusterState {
		v.InitialCluster = initialCluster
		md.cfg.ClusterState[id] = v
	}
	if err = md.cfg.ValidateAndSetDefaults(); err != nil {
		return err
	}

	// SCP to each EC2 instance
	md.lg.Info("deploying etcd",
		zap.String("initial-cluster", initialCluster),
	)

	md.lg.Info("deployed etcd",
		zap.String("initial-cluster", initialCluster),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	return md.cfg.ValidateAndSetDefaults()
}

func (md *embedded) CheckHealth() map[string]etcdtester.Health {
	md.mu.RLock()
	defer md.mu.RUnlock()

	return nil
}

func (md *embedded) IDToClientURL() (rm map[string]string) {
	md.mu.RLock()
	defer md.mu.RUnlock()

	rm = make(map[string]string, len(md.cfg.ClusterState))
	for k, v := range md.cfg.ClusterState {
		rm[k] = v.AdvertiseClientURLs
	}
	return rm
}

func (md *embedded) Stop(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) Restart(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) Terminate() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	// TODO: get logs

	return nil
}

func (md *embedded) MemberAdd(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) MemberRemove(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}
