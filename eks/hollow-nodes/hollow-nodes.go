// Package hollownodes implements Hollow Nodes.
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
//
// The purpose is to make it easy to run on EKS.
// ref. https://github.com/kubernetes/kubernetes/blob/master/test/kubemark/start-kubemark.sh
//
package hollownodes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelet_app "k8s.io/kubernetes/cmd/kubelet/app"
	kubelet_options "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/kubelet"
	kubelet_config "k8s.io/kubernetes/pkg/kubelet/apis/config"
	cadvisor_test "k8s.io/kubernetes/pkg/kubelet/cadvisor/testing"
	kubelet_cm "k8s.io/kubernetes/pkg/kubelet/cm"
	container_test "k8s.io/kubernetes/pkg/kubelet/container/testing"
	kubelet_remote "k8s.io/kubernetes/pkg/kubelet/remote"
	kubelet_remote_fake "k8s.io/kubernetes/pkg/kubelet/remote/fake"
	kubelet_types "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/util/oom"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/cephfs"
	"k8s.io/kubernetes/pkg/volume/configmap"
	"k8s.io/kubernetes/pkg/volume/downwardapi"
	"k8s.io/kubernetes/pkg/volume/emptydir"
	"k8s.io/kubernetes/pkg/volume/fc"
	"k8s.io/kubernetes/pkg/volume/flocker"
	"k8s.io/kubernetes/pkg/volume/git_repo"
	"k8s.io/kubernetes/pkg/volume/glusterfs"
	"k8s.io/kubernetes/pkg/volume/hostpath"
	"k8s.io/kubernetes/pkg/volume/iscsi"
	"k8s.io/kubernetes/pkg/volume/local"
	"k8s.io/kubernetes/pkg/volume/nfs"
	"k8s.io/kubernetes/pkg/volume/portworx"
	"k8s.io/kubernetes/pkg/volume/projected"
	"k8s.io/kubernetes/pkg/volume/quobyte"
	"k8s.io/kubernetes/pkg/volume/rbd"
	"k8s.io/kubernetes/pkg/volume/scaleio"
	"k8s.io/kubernetes/pkg/volume/secret"
	"k8s.io/kubernetes/pkg/volume/storageos"
	"k8s.io/kubernetes/pkg/volume/util/hostutil"
	"k8s.io/kubernetes/pkg/volume/util/subpath"
	"k8s.io/utils/mount"
)

// KubeletConfig is the kubelet configuration.
// TODO: contribute to upstream, revisit EKS auth provider
// ref. https://github.com/kubernetes/kubernetes/pull/81796
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
type KubeletConfig struct {
	Logger *zap.Logger
	Stopc  chan struct{}
	Donec  chan struct{}

	Client k8s_client.EKS

	NodeName     string
	NodeLabels   map[string]string
	MaxOpenFiles int64
}

type hollowKubelet struct {
	cfg               KubeletConfig
	f                 *kubelet_options.KubeletFlags
	c                 *kubelet_config.KubeletConfiguration
	fakeRemoteRuntime *kubelet_remote_fake.RemoteRuntime
	deps              *kubelet.Dependencies
}

type Kubelet interface {
	Start()
	Stop()
}

// NewKubelet creates a new Kubelet.
// ref. "pkg/kubemark.GetHollowKubeletConfig"
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
// "pkg/kubelet".fastStatusUpdateOnce runs updates every 100ms
func NewKubelet(cfg KubeletConfig) (Kubelet, error) {
	cfg.Logger.Info("creating kubelet flags")
	f := kubelet_options.NewKubeletFlags()

	cfg.Logger.Info("creating kubelet configuration")
	c, err := kubelet_options.NewKubeletConfiguration()
	if err != nil {
		return nil, err
	}

	rootDir := fileutil.MkTmpDir(os.TempDir(), "hollow-kubelet")
	f.RootDirectory = rootDir

	// podFilesDir := fileutil.MkTmpDir(os.TempDir(), "static-pods")
	// c.StaticPodPath = podFilesDir
	c.StaticPodPath = ""
	c.StaticPodURL = ""

	// f.EnableServer = true
	// c.Port = int32(cfg.kubeletPort)
	// c.ReadOnlyPort = int32(cfg.kubeletReadOnlyPort)
	f.ReallyCrashForTesting = true
	f.EnableServer = false
	c.Port = 0
	c.ReadOnlyPort = 0

	f.HostnameOverride = cfg.NodeName
	f.NodeLabels = cfg.NodeLabels

	f.MinimumGCAge = metav1.Duration{Duration: 1 * time.Minute}
	f.MaxContainerCount = 1
	f.MaxPerPodContainerCount = 1
	f.ContainerRuntimeOptions.ContainerRuntime = kubelet_types.RemoteContainerRuntime
	f.RegisterNode = true
	f.RegisterSchedulable = true
	f.ProviderID = fmt.Sprintf("kubemark://%v", cfg.NodeName)

	c.Address = "0.0.0.0" /* bind address */
	c.Authentication.Anonymous.Enabled = true

	c.FileCheckFrequency.Duration = 20 * time.Second
	c.HTTPCheckFrequency.Duration = 20 * time.Second

	// default is 10-second
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	c.NodeStatusUpdateFrequency.Duration = 10 * time.Second

	// default is 5-minute
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	// "tryUpdateNodeStatus" patches node status
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kubelet_node_status.go
	c.NodeStatusReportFrequency.Duration = 5 * time.Minute

	// node lease renew interval
	// default is 40 seconds
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	c.NodeLeaseDurationSeconds = 40

	c.SyncFrequency.Duration = 10 * time.Second
	c.EvictionPressureTransitionPeriod.Duration = 5 * time.Minute
	c.MaxPods = 1
	c.PodsPerCore = 1
	c.ClusterDNS = []string{}
	c.ImageGCHighThresholdPercent = 90
	c.ImageGCLowThresholdPercent = 80
	c.VolumeStatsAggPeriod.Duration = time.Minute
	c.CgroupRoot = ""
	c.CPUCFSQuota = true
	c.EnableControllerAttachDetach = false
	c.EnableDebuggingHandlers = true
	c.CgroupsPerQOS = false
	// hairpin-veth is used to allow hairpin packets. Note that this deviates from
	// what the "real" kubelet currently does, because there's no way to
	// set promiscuous mode on docker0.
	c.HairpinMode = kubelet_config.HairpinVeth

	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host
	// TODO: increase if we run this in remote
	c.MaxOpenFiles = 1000000

	c.RegistryBurst = 10
	c.RegistryPullQPS = 5.0
	c.ResolverConfig = kubelet_types.ResolvConfDefault
	c.KubeletCgroups = "/kubelet"
	c.SerializeImagePulls = true
	c.SystemCgroups = ""
	c.ProtectKernelDefaults = false

	cadvisorInterface := &cadvisor_test.Fake{
		NodeName: cfg.NodeName,
	}
	containerManager := kubelet_cm.NewStubContainerManager()

	endpoint, err := kubelet_remote_fake.GenerateEndpoint()
	if err != nil {
		cfg.Logger.Warn("failed to generate fake endpoint", zap.Error(err))
		return nil, err
	}
	fakeRemoteRuntime := kubelet_remote_fake.NewFakeRemoteRuntime()
	if err = fakeRemoteRuntime.Start(endpoint); err != nil {
		cfg.Logger.Warn("failed to start fake runtime", zap.Error(err))
		return nil, err
	}
	runtimeService, err := kubelet_remote.NewRemoteRuntimeService(endpoint, 15*time.Second)
	if err != nil {
		cfg.Logger.Warn("failed to init runtime service", zap.Error(err))
		return nil, err
	}

	return &hollowKubelet{
		cfg:               cfg,
		f:                 f,
		c:                 c,
		fakeRemoteRuntime: fakeRemoteRuntime,
		deps: &kubelet.Dependencies{
			KubeClient:           cfg.Client.KubernetesClientSet(),
			HeartbeatClient:      cfg.Client.KubernetesClientSet(),
			RemoteRuntimeService: runtimeService,
			RemoteImageService:   fakeRemoteRuntime.ImageService,
			CAdvisorInterface:    cadvisorInterface,

			// TODO: mock
			Cloud: nil,

			OSInterface:      &container_test.FakeOS{},
			ContainerManager: containerManager,
			VolumePlugins:    volumePlugins(),

			TLSOptions: nil,

			OOMAdjuster: oom.NewFakeOOMAdjuster(),
			Mounter:     &mount.FakeMounter{},
			Subpather:   &subpath.FakeSubpath{},
			HostUtil:    hostutil.NewFakeHostUtil(nil),
		},
	}, nil
}

func (k *hollowKubelet) Start() {
	k.cfg.Logger.Info("running a new kubelet", zap.String("node-name", k.cfg.NodeName))
	if err := kubelet_app.RunKubelet(&kubelet_options.KubeletServer{
		KubeletFlags:         *k.f,
		KubeletConfiguration: *k.c,
	}, k.deps, false); err != nil {
		k.cfg.Logger.Warn("failed to run kubelet", zap.Error(err))
		return
	}
	select {
	case <-k.cfg.Stopc:
		k.cfg.Logger.Info("kubelet run stopped", zap.String("node-name", k.cfg.NodeName))
		return
	case <-k.cfg.Donec:
		k.cfg.Logger.Info("kubelet run canceled", zap.String("node-name", k.cfg.NodeName))
		return
	}
}

func (k *hollowKubelet) Stop() {
	k.cfg.Logger.Info("stopping hollow node", zap.String("node-name", k.cfg.NodeName))
	k.fakeRemoteRuntime.Stop()
	k.cfg.Logger.Info("stopped hollow node", zap.String("node-name", k.cfg.NodeName))
}

func volumePlugins() []volume.VolumePlugin {
	// csi.ProbeVolumePlugins not working

	allPlugins := []volume.VolumePlugin{}
	allPlugins = append(allPlugins, emptydir.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, git_repo.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, hostpath.ProbeVolumePlugins(volume.VolumeConfig{})...)
	allPlugins = append(allPlugins, nfs.ProbeVolumePlugins(volume.VolumeConfig{})...)
	allPlugins = append(allPlugins, secret.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, iscsi.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, glusterfs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, rbd.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, quobyte.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, cephfs.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, downwardapi.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, fc.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, flocker.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, configmap.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, projected.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, portworx.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, scaleio.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, local.ProbeVolumePlugins()...)
	allPlugins = append(allPlugins, storageos.ProbeVolumePlugins()...)
	// allPlugins = append(allPlugins, csi.ProbeVolumePlugins()...)

	return allPlugins
}

// NodeGroup represents a set of hollow node objects.
type NodeGroup interface {
	Start() error
	Stop()
	CheckNodes() (readyNodes []string, createdNodes []string, err error)
}

// NodeGroupConfig is the hollow nodes configuration.
type NodeGroupConfig struct {
	Logger         *zap.Logger
	Stopc          chan struct{}
	Client         k8s_client.EKS
	Nodes          int
	NodeNamePrefix string
	NodeLabels     map[string]string
	MaxOpenFiles   int64
}

type nodeGroup struct {
	mu sync.Mutex

	cfg NodeGroupConfig

	donec          chan struct{}
	donecCloseOnce *sync.Once

	kubelets []Kubelet
}

// CreateNodeGroup creates a new hollow node group.
func CreateNodeGroup(cfg NodeGroupConfig) (ng NodeGroup, err error) {
	return &nodeGroup{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}, nil
}

func (ng *nodeGroup) Start() (err error) {
	ng.mu.Lock()
	defer ng.mu.Unlock()

	ng.kubelets = make([]Kubelet, ng.cfg.Nodes)

	ng.cfg.Logger.Info("creating node group with hollow nodes",
		zap.Int("nodes", ng.cfg.Nodes),
		zap.String("node-name-prefix", ng.cfg.NodeNamePrefix),
		zap.Any("node-labels", ng.cfg.NodeLabels),
	)
	for i := 0; i < ng.cfg.Nodes; i++ {
		ng.kubelets[i], err = NewKubelet(KubeletConfig{
			Logger:       ng.cfg.Logger,
			Stopc:        ng.cfg.Stopc,
			Donec:        ng.donec,
			Client:       ng.cfg.Client,
			NodeName:     ng.cfg.NodeNamePrefix + randutil.String(10),
			NodeLabels:   ng.cfg.NodeLabels,
			MaxOpenFiles: ng.cfg.MaxOpenFiles,
		})
		if err != nil {
			return err
		}
	}
	ng.cfg.Logger.Info("created node group with hollow nodes", zap.Int("nodes", ng.cfg.Nodes))

	ng.cfg.Logger.Info("starting hollow node group")
	for _, node := range ng.kubelets {
		go node.Start()
	}
	ng.cfg.Logger.Info("started hollow node group")
	return nil
}

func (ng *nodeGroup) Stop() {
	ng.mu.Lock()
	defer ng.mu.Unlock()

	ng.cfg.Logger.Info("stopping hollow node group")
	ng.donecCloseOnce.Do(func() {
		close(ng.donec)
	})
	for _, node := range ng.kubelets {
		node.Stop()
	}
	ng.cfg.Logger.Info("stopped hollow node group")
}

func (ng *nodeGroup) CheckNodes() (readyNodes []string, createdNodes []string, err error) {
	ng.mu.Lock()
	defer ng.mu.Unlock()
	return ng.checkNodes()
}

func (ng *nodeGroup) checkNodes() (readyNodes []string, createdNodes []string, err error) {
	waitDur := 5 * time.Minute
	ng.cfg.Logger.Info("checking nodes readiness", zap.Duration("wait", waitDur))
	retryStart, ready := time.Now(), false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ng.cfg.Stopc:
			ng.cfg.Logger.Info("checking nodes aborted")
			return nil, nil, nil
		case <-ng.donec:
			ng.cfg.Logger.Info("checking nodes aborted")
			return nil, nil, nil
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodes, err := ng.cfg.Client.KubernetesClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ng.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items := nodes.Items

		readyNodes, createdNodes = make([]string, 0), make([]string, 0)
		readies := 0
		for _, node := range items {
			nodeName := node.GetName()
			if strings.HasPrefix(nodeName, ng.cfg.NodeNamePrefix) {
				continue
			}
			labels := node.GetLabels()
			notMatch := false
			for k, v1 := range ng.cfg.NodeLabels {
				v2, ok := labels[k]
				if !ok {
					notMatch = true
					break
				}
				if v1 != v2 {
					notMatch = true
					break
				}
			}
			if notMatch {
				ng.cfg.Logger.Warn("node labels not match", zap.String("node-name", nodeName), zap.Any("expected", ng.cfg.NodeLabels), zap.Any("got", labels))
				continue
			}

			ng.cfg.Logger.Info("checking node readiness", zap.String("name", nodeName))
			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				createdNodes = append(createdNodes, nodeName)
				if cond.Type != v1.NodeReady {
					continue
				}
				ng.cfg.Logger.Info("checked node readiness",
					zap.String("name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				readyNodes = append(readyNodes, nodeName)
				readies++
				break
			}
		}
		ng.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", ng.cfg.Nodes),
		)

		if readies >= ng.cfg.Nodes {
			ready = true
			break
		}
	}
	if !ready {
		ng.cfg.Logger.Info("not all hollow nodes are ready", zap.Int("ready-nodes", len(readyNodes)), zap.Int("created-nodes", len(createdNodes)), zap.Int("desired-nodes", ng.cfg.Nodes))
		return nil, nil, errors.New("NG 'fake' not ready")
	}

	ng.cfg.Logger.Info("checked hollow node group", zap.Int("ready-nodes", len(readyNodes)), zap.Int("created-nodes", len(createdNodes)), zap.Int("desired-nodes", ng.cfg.Nodes))
	return readyNodes, createdNodes, err
}
