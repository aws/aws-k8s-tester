package hollownodes

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
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

// TODO: contribute to upstream, revisit EKS auth provider
// ref. https://github.com/kubernetes/kubernetes/pull/81796
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
type kubeletConfig struct {
	lg    *zap.Logger
	cli   k8s_client.EKS
	stopc chan struct{}
	donec chan struct{}

	nodeName     string
	nodeLabels   map[string]string
	maxOpenFiles int64
}

type hollowKubelet struct {
	cfg               kubeletConfig
	f                 *kubelet_options.KubeletFlags
	c                 *kubelet_config.KubeletConfiguration
	fakeRemoteRuntime *kubelet_remote_fake.RemoteRuntime
	deps              *kubelet.Dependencies
}

// ref. "pkg/kubemark.GetHollowKubeletConfig"
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
// "pkg/kubelet".fastStatusUpdateOnce runs updates every 100ms
func newKubelet(cfg kubeletConfig) (*hollowKubelet, error) {
	cfg.lg.Info("creating kubelet flags")
	f := kubelet_options.NewKubeletFlags()

	cfg.lg.Info("creating kubelet configuration")
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

	f.HostnameOverride = cfg.nodeName
	f.NodeLabels = cfg.nodeLabels

	f.MinimumGCAge = metav1.Duration{Duration: 1 * time.Minute}
	f.MaxContainerCount = 1
	f.MaxPerPodContainerCount = 1
	f.ContainerRuntimeOptions.ContainerRuntime = kubelet_types.RemoteContainerRuntime
	f.RegisterNode = true
	f.RegisterSchedulable = true
	f.ProviderID = fmt.Sprintf("kubemark://%v", cfg.nodeName)

	c.Address = "0.0.0.0" /* bind address */
	c.Authentication.Anonymous.Enabled = true

	c.FileCheckFrequency.Duration = 20 * time.Second
	c.HTTPCheckFrequency.Duration = 20 * time.Second
	c.NodeStatusUpdateFrequency.Duration = 10 * time.Second
	c.NodeStatusReportFrequency.Duration = 5 * time.Minute
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
	// TOOD: increase if we run this in remote
	c.MaxOpenFiles = 1000000

	c.RegistryBurst = 10
	c.RegistryPullQPS = 5.0
	c.ResolverConfig = kubelet_types.ResolvConfDefault
	c.KubeletCgroups = "/kubelet"
	c.SerializeImagePulls = true
	c.SystemCgroups = ""
	c.ProtectKernelDefaults = false

	cadvisorInterface := &cadvisor_test.Fake{
		NodeName: cfg.nodeName,
	}
	containerManager := kubelet_cm.NewStubContainerManager()

	endpoint, err := kubelet_remote_fake.GenerateEndpoint()
	if err != nil {
		cfg.lg.Warn("failed to generate fake endpoint", zap.Error(err))
		return nil, err
	}
	fakeRemoteRuntime := kubelet_remote_fake.NewFakeRemoteRuntime()
	if err = fakeRemoteRuntime.Start(endpoint); err != nil {
		cfg.lg.Warn("failed to start fake runtime", zap.Error(err))
		return nil, err
	}
	runtimeService, err := kubelet_remote.NewRemoteRuntimeService(endpoint, 15*time.Second)
	if err != nil {
		cfg.lg.Warn("failed to init runtime service", zap.Error(err))
		return nil, err
	}

	return &hollowKubelet{
		cfg:               cfg,
		f:                 f,
		c:                 c,
		fakeRemoteRuntime: fakeRemoteRuntime,
		deps: &kubelet.Dependencies{
			KubeClient:           cfg.cli.KubernetesClientSet(),
			HeartbeatClient:      cfg.cli.KubernetesClientSet(),
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

func (k *hollowKubelet) run() {
	k.cfg.lg.Info("running a new kubelet", zap.String("node-name", k.cfg.nodeName))
	if err := kubelet_app.RunKubelet(&kubelet_options.KubeletServer{
		KubeletFlags:         *k.f,
		KubeletConfiguration: *k.c,
	}, k.deps, false); err != nil {
		k.cfg.lg.Warn("failed to run kubelet", zap.Error(err))
		return
	}
	select {
	case <-k.cfg.stopc:
		k.cfg.lg.Info("kubelet run stopped", zap.String("node-name", k.cfg.nodeName))
		return
	case <-k.cfg.donec:
		k.cfg.lg.Info("kubelet run canceled", zap.String("node-name", k.cfg.nodeName))
		return
	}
}

func (k *hollowKubelet) stop() {
	k.cfg.lg.Info("stopping hollow node", zap.String("node-name", k.cfg.nodeName))
	k.fakeRemoteRuntime.Stop()
	k.cfg.lg.Info("stopped hollow node", zap.String("node-name", k.cfg.nodeName))
}
