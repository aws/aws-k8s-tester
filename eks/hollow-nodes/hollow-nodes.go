// Package hollownodes implements hollow nodes.
//
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_proxy.go
//
// The purpose is to make it easy to run on EKS.
// ref. https://github.com/kubernetes/kubernetes/blob/master/test/kubemark/start-kubemark.sh
//
package hollownodes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	proxy_app "k8s.io/kubernetes/cmd/kube-proxy/app"
	kubelet_app "k8s.io/kubernetes/cmd/kubelet/app"
	kubelet_options "k8s.io/kubernetes/cmd/kubelet/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core"
	pkg_kubelet "k8s.io/kubernetes/pkg/kubelet"
	kubelet_config "k8s.io/kubernetes/pkg/kubelet/apis/config"
	cadvisor_test "k8s.io/kubernetes/pkg/kubelet/cadvisor/testing"
	kubelet_cm "k8s.io/kubernetes/pkg/kubelet/cm"
	container_test "k8s.io/kubernetes/pkg/kubelet/container/testing"
	kubelet_remote "k8s.io/kubernetes/pkg/kubelet/remote"
	kubelet_remote_fake "k8s.io/kubernetes/pkg/kubelet/remote/fake"
	kubelet_types "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/proxy"
	proxy_config "k8s.io/kubernetes/pkg/proxy/config"
	proxy_iptables "k8s.io/kubernetes/pkg/proxy/iptables"
	utils_iptables "k8s.io/kubernetes/pkg/proxy/util/iptables"
	iptables_testing "k8s.io/kubernetes/pkg/util/iptables/testing"
	util_node "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/pkg/util/oom"
	sysctl_testing "k8s.io/kubernetes/pkg/util/sysctl/testing"
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
	exec_testing "k8s.io/utils/exec/testing"
	"k8s.io/utils/mount"
	util_pointer "k8s.io/utils/pointer"
)

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

	// Remote is true if run in remote nodes (inside Pod).
	Remote bool
}

type nodeGroup struct {
	mu sync.Mutex

	cfg NodeGroupConfig

	donec          chan struct{}
	donecCloseOnce *sync.Once

	kubelets    []kubelet
	kubeProxies []kubeProxy
}

// CreateNodeGroup creates a new hollow node group.
func CreateNodeGroup(cfg NodeGroupConfig) NodeGroup {
	return &nodeGroup{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}
}

func (ng *nodeGroup) Start() (err error) {
	ng.mu.Lock()
	defer ng.mu.Unlock()

	ng.kubelets = make([]kubelet, ng.cfg.Nodes)
	ng.kubeProxies = make([]kubeProxy, ng.cfg.Nodes)

	ng.cfg.Logger.Info("creating node group with hollow nodes",
		zap.Int("nodes", ng.cfg.Nodes),
		zap.String("node-name-prefix", ng.cfg.NodeNamePrefix),
		zap.Any("node-labels", ng.cfg.NodeLabels),
	)
	for i := 0; i < ng.cfg.Nodes; i++ {
		// Cluster Autoscaler's Kubemark integration requires that the podname and node name are equal.
		// To support this, the first node will be named identically to the pod and each subsequent node suffixed with index.
		// For ClusterAutoscaler configurations, the #Nodes must be 1.
		nodeName := ng.cfg.NodeNamePrefix
		if i > 0 {
			nodeName += string(i)
		}

		ng.kubelets[i], ng.kubeProxies[i], err = newNode(nodeConfig{
			lg:           ng.cfg.Logger,
			stopc:        ng.cfg.Stopc,
			donec:        ng.donec,
			cli:          ng.cfg.Client,
			nodeName:     nodeName,
			nodeLabels:   ng.cfg.NodeLabels,
			maxOpenFiles: ng.cfg.MaxOpenFiles,
			remote:       ng.cfg.Remote,
		})
		if err != nil {
			ng.cfg.Logger.Warn("failed to create hollow node", zap.Error(err))
			return err
		}
	}
	ng.cfg.Logger.Info("created node group with hollow nodes", zap.Int("nodes", ng.cfg.Nodes))

	ng.cfg.Logger.Info("starting hollow node group")
	for idx := range ng.kubelets {
		go ng.kubelets[idx].Start()
		go ng.kubeProxies[idx].Start()
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
	for idx := range ng.kubeProxies {
		ng.kubeProxies[idx].Stop()
		ng.kubelets[idx].Stop()
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
			if !strings.HasPrefix(nodeName, ng.cfg.NodeNamePrefix) {
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
				ng.cfg.Logger.Info("node labels not match", zap.String("node-name", nodeName), zap.Any("expected", ng.cfg.NodeLabels), zap.Any("got", labels))
				continue
			}

			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				createdNodes = append(createdNodes, nodeName)
				if cond.Type != v1.NodeReady {
					continue
				}
				ng.cfg.Logger.Info("node is ready!",
					zap.String("name", nodeName),
					zap.String("status-type", fmt.Sprintf("%s", cond.Type)),
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

// nodeConfig is the kubelet configuration.
// TODO: contribute to upstream, revisit EKS auth provider
// ref. https://github.com/kubernetes/kubernetes/pull/81796
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
type nodeConfig struct {
	lg    *zap.Logger
	stopc chan struct{}
	donec chan struct{}

	cli k8s_client.EKS

	nodeName     string
	nodeLabels   map[string]string
	maxOpenFiles int64

	remote bool
}

type hollowKubelet struct {
	cfg nodeConfig

	kubeletFlags      *kubelet_options.KubeletFlags
	kubeletConfig     *kubelet_config.KubeletConfiguration
	fakeRemoteRuntime *kubelet_remote_fake.RemoteRuntime
	deps              *pkg_kubelet.Dependencies
}

// kubelet defines hollow kubelet interface.
type kubelet interface {
	Start()
	Stop()
}

type hollowKubeProxy struct {
	cfg    nodeConfig
	server *proxy_app.ProxyServer
}

// kubeProxy defines hollow kube-proxy interface.
type kubeProxy interface {
	Start()
	Stop()
}

// newNode creates a new kubelet and kube-proxy.
// ref. "pkg/kubemark.GetHollowKubeletConfig"
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
// "pkg/kubelet".fastStatusUpdateOnce runs updates every 100ms
func newNode(cfg nodeConfig) (kubelet, kubeProxy, error) {
	cfg.lg.Info("creating kubelet flags")
	kubeletFlags := kubelet_options.NewKubeletFlags()

	cfg.lg.Info("creating kubelet configuration")
	kubeletConfig, err := kubelet_options.NewKubeletConfiguration()
	if err != nil {
		return nil, nil, err
	}

	rootDir := fileutil.MkTmpDir(os.TempDir(), "hollow-kubelet")
	kubeletFlags.RootDirectory = rootDir

	// podFilesDir := fileutil.MkTmpDir(os.TempDir(), "static-pods")
	// c.StaticPodPath = podFilesDir
	kubeletConfig.StaticPodPath = ""
	kubeletConfig.StaticPodURL = ""

	// f.EnableServer = true
	// c.Port = int32(cfg.kubeletPort)
	// c.ReadOnlyPort = int32(cfg.kubeletReadOnlyPort)
	kubeletFlags.ReallyCrashForTesting = true
	kubeletFlags.EnableServer = false
	kubeletConfig.Port = 0
	kubeletConfig.ReadOnlyPort = 0

	kubeletFlags.HostnameOverride = cfg.nodeName
	kubeletFlags.NodeLabels = cfg.nodeLabels
	kubeletFlags.RegisterWithTaints = []core.Taint{{Key: "provider", Effect: "NoSchedule", Value: "kubemark"}}

	kubeletFlags.MinimumGCAge = metav1.Duration{Duration: 1 * time.Minute}
	kubeletFlags.MaxContainerCount = 1
	kubeletFlags.MaxPerPodContainerCount = 1
	kubeletFlags.ContainerRuntimeOptions.ContainerRuntime = kubelet_types.RemoteContainerRuntime
	kubeletFlags.RegisterNode = true
	kubeletFlags.RegisterSchedulable = true
	kubeletFlags.ProviderID = fmt.Sprintf("kubemark://%v", cfg.nodeName)

	kubeletConfig.Address = "0.0.0.0" /* bind address */
	kubeletConfig.Authentication.Anonymous.Enabled = true

	kubeletConfig.FileCheckFrequency.Duration = 20 * time.Second
	kubeletConfig.HTTPCheckFrequency.Duration = 20 * time.Second

	// default is 10-second
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	kubeletConfig.NodeStatusUpdateFrequency.Duration = 10 * time.Second

	// default is 5-minute
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	// "tryUpdateNodeStatus" patches node status
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/kubelet_node_status.go
	kubeletConfig.NodeStatusReportFrequency.Duration = 5 * time.Minute

	// node lease renew interval
	// default is 40 seconds
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/apis/config/v1beta1/defaults.go
	kubeletConfig.NodeLeaseDurationSeconds = 40

	kubeletConfig.SyncFrequency.Duration = 10 * time.Second
	kubeletConfig.EvictionPressureTransitionPeriod.Duration = 5 * time.Minute
	kubeletConfig.MaxPods = 10
	kubeletConfig.ClusterDNS = []string{}
	kubeletConfig.ImageGCHighThresholdPercent = 90
	kubeletConfig.ImageGCLowThresholdPercent = 80
	kubeletConfig.VolumeStatsAggPeriod.Duration = time.Minute
	kubeletConfig.CgroupRoot = ""
	kubeletConfig.CPUCFSQuota = true
	kubeletConfig.EnableControllerAttachDetach = false
	kubeletConfig.EnableDebuggingHandlers = true
	kubeletConfig.CgroupsPerQOS = false
	// hairpin-veth is used to allow hairpin packets. Note that this deviates from
	// what the "real" kubelet currently does, because there's no way to
	// set promiscuous mode on docker0.
	kubeletConfig.HairpinMode = kubelet_config.HairpinVeth

	// "cmd/kubelet/app.rlimit.SetNumFiles(MaxOpenFiles)" sets this for the host
	// TODO: increase if we run this in remote
	kubeletConfig.MaxOpenFiles = 1000000

	kubeletConfig.RegistryBurst = 10
	kubeletConfig.RegistryPullQPS = 5.0
	kubeletConfig.ResolverConfig = kubelet_types.ResolvConfDefault
	kubeletConfig.KubeletCgroups = "/kubelet"
	kubeletConfig.SerializeImagePulls = true
	kubeletConfig.SystemCgroups = ""
	kubeletConfig.ProtectKernelDefaults = false

	cadvisorInterface := &cadvisor_test.Fake{NodeName: cfg.nodeName}
	containerManager := kubelet_cm.NewStubContainerManager()

	fakeEndpoint, err := kubelet_remote_fake.GenerateEndpoint()
	if err != nil {
		cfg.lg.Warn("failed to generate fake endpoint", zap.Error(err))
		return nil, nil, err
	}
	fakeRemoteRuntime := kubelet_remote_fake.NewFakeRemoteRuntime()
	if err = fakeRemoteRuntime.Start(fakeEndpoint); err != nil {
		cfg.lg.Warn("failed to start fake runtime", zap.Error(err))
		return nil, nil, err
	}
	fakeRuntimeService, err := kubelet_remote.NewRemoteRuntimeService(fakeEndpoint, 15*time.Second)
	if err != nil {
		cfg.lg.Warn("failed to init runtime service", zap.Error(err))
		return nil, nil, err
	}
	hollowKube := &hollowKubelet{
		cfg: cfg,

		kubeletFlags:  kubeletFlags,
		kubeletConfig: kubeletConfig,

		fakeRemoteRuntime: fakeRemoteRuntime,

		deps: &pkg_kubelet.Dependencies{
			KubeClient:      cfg.cli.KubernetesClientSet(),
			HeartbeatClient: cfg.cli.KubernetesClientSet(),

			RemoteRuntimeService: fakeRuntimeService,
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
	}

	fakeIptables := iptables_testing.NewFake()
	fakeSysctl := sysctl_testing.NewFake()
	fakeExec := &exec_testing.FakeExec{
		LookPathFunc: func(_ string) (string, error) { return "", errors.New("fake execer") },
	}
	fakeEventBroadcaster := record.NewBroadcaster()
	fakeRecorder := fakeEventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "kube-proxy", Host: cfg.nodeName})

	var proxier proxy.Provider
	if cfg.remote {
		nodeIP := util_node.GetNodeIP(cfg.cli.KubernetesClientSet(), cfg.nodeName)
		if nodeIP == nil {
			cfg.lg.Warn("failed to find node IP; assuming 127.0.0.1", zap.String("node-name", cfg.nodeName))
			nodeIP = net.ParseIP("127.0.0.1")
		} else {
			cfg.lg.Warn("found node IP", zap.String("node-name", cfg.nodeName), zap.String("node-ip", fmt.Sprintf("%+v", nodeIP)))
		}
		proxier, err = proxy_iptables.NewProxier(
			fakeIptables,
			fakeSysctl,
			fakeExec,
			30*time.Second,
			0,
			false,
			0,
			utils_iptables.NewNoOpLocalDetector(),
			cfg.nodeName,
			nodeIP,
			fakeRecorder,
			nil,
			[]string{},
		)
		if err != nil {
			cfg.lg.Warn("failed to create a new proxier", zap.String("node-name", cfg.nodeName), zap.Error(err))
			proxier = nil
		}
	}
	if proxier == nil {
		cfg.lg.Info("use fake proxy", zap.String("node-name", cfg.nodeName))
		proxier = &fakeProxy{}
	}
	hollowPxy := &hollowKubeProxy{
		cfg: cfg,
		server: &proxy_app.ProxyServer{
			Client:       cfg.cli.KubernetesClientSet(),
			EventClient:  cfg.cli.KubernetesClientSet().CoreV1(),
			IptInterface: fakeIptables,
			Proxier:      proxier,
			Broadcaster:  fakeEventBroadcaster,
			Recorder:     fakeRecorder,
			ProxyMode:    "fake",
			NodeRef: &v1.ObjectReference{
				Kind:      "Node",
				Name:      cfg.nodeName,
				UID:       types.UID(cfg.nodeName),
				Namespace: "",
			},
			OOMScoreAdj:      util_pointer.Int32Ptr(0),
			ConfigSyncPeriod: 30 * time.Second,
		},
	}
	return hollowKube, hollowPxy, nil
}

func (k *hollowKubelet) Start() {
	// non-blocking run
	k.cfg.lg.Info("starting hollow node kubelet", zap.String("node-name", k.cfg.nodeName))
	if err := kubelet_app.RunKubelet(&kubelet_options.KubeletServer{
		KubeletFlags:         *k.kubeletFlags,
		KubeletConfiguration: *k.kubeletConfig,
	}, k.deps, false); err != nil {
		k.cfg.lg.Warn("failed to run kubelet", zap.Error(err))
		return
	}
	k.cfg.lg.Info("started hollow node kubelet", zap.String("node-name", k.cfg.nodeName))

	select {
	case <-k.cfg.stopc:
		k.cfg.lg.Info("hollow node kubelet stopped", zap.String("node-name", k.cfg.nodeName))
		return
	case <-k.cfg.donec:
		k.cfg.lg.Info("hollow node kubelet canceled", zap.String("node-name", k.cfg.nodeName))
		return
	}
}

func (k *hollowKubelet) Stop() {
	k.cfg.lg.Info("stopping hollow node kubelet", zap.String("node-name", k.cfg.nodeName))
	k.fakeRemoteRuntime.Stop()
	k.cfg.lg.Info("stopped hollow node kubelet", zap.String("node-name", k.cfg.nodeName))
}

func (k *hollowKubeProxy) Start() {
	// blocking run
	k.cfg.lg.Info("starting hollow node kube-proxy", zap.String("node-name", k.cfg.nodeName))
	if err := k.server.Run(); err != nil {
		k.cfg.lg.Warn("failed to run kube-proxy", zap.String("node-name", k.cfg.nodeName), zap.Error(err))
		return
	}
	k.cfg.lg.Info("started hollow node kube-proxy", zap.String("node-name", k.cfg.nodeName))

	select {
	case <-k.cfg.stopc:
		k.cfg.lg.Info("hollow node kube-proxy stopped", zap.String("node-name", k.cfg.nodeName))
		return
	case <-k.cfg.donec:
		k.cfg.lg.Info("hollow node kube-proxy canceled", zap.String("node-name", k.cfg.nodeName))
		return
	}
}

func (k *hollowKubeProxy) Stop() {
	k.cfg.lg.Info("stopping hollow node kube-proxy", zap.String("node-name", k.cfg.nodeName))
	// no-op
	k.cfg.lg.Info("stopped hollow node kube-proxy", zap.String("node-name", k.cfg.nodeName))
}

type fakeProxy struct {
	proxy_config.NoopEndpointSliceHandler
	proxy_config.NoopNodeHandler
}

func (*fakeProxy) Sync() {}
func (*fakeProxy) SyncLoop() {
	select {}
}
func (*fakeProxy) OnServiceAdd(service *v1.Service)                        {}
func (*fakeProxy) OnServiceUpdate(oldService, service *v1.Service)         {}
func (*fakeProxy) OnServiceDelete(service *v1.Service)                     {}
func (*fakeProxy) OnServiceSynced()                                        {}
func (*fakeProxy) OnEndpointsAdd(endpoints *v1.Endpoints)                  {}
func (*fakeProxy) OnEndpointsUpdate(oldEndpoints, endpoints *v1.Endpoints) {}
func (*fakeProxy) OnEndpointsDelete(endpoints *v1.Endpoints)               {}
func (*fakeProxy) OnEndpointsSynced()                                      {}

func volumePlugins() (pgs []volume.VolumePlugin) {
	pgs = append(pgs, emptydir.ProbeVolumePlugins()...)
	pgs = append(pgs, git_repo.ProbeVolumePlugins()...)
	pgs = append(pgs, hostpath.ProbeVolumePlugins(volume.VolumeConfig{})...)
	pgs = append(pgs, nfs.ProbeVolumePlugins(volume.VolumeConfig{})...)
	pgs = append(pgs, secret.ProbeVolumePlugins()...)
	pgs = append(pgs, iscsi.ProbeVolumePlugins()...)
	pgs = append(pgs, glusterfs.ProbeVolumePlugins()...)
	pgs = append(pgs, rbd.ProbeVolumePlugins()...)
	pgs = append(pgs, quobyte.ProbeVolumePlugins()...)
	pgs = append(pgs, cephfs.ProbeVolumePlugins()...)
	pgs = append(pgs, downwardapi.ProbeVolumePlugins()...)
	pgs = append(pgs, fc.ProbeVolumePlugins()...)
	pgs = append(pgs, flocker.ProbeVolumePlugins()...)
	pgs = append(pgs, configmap.ProbeVolumePlugins()...)
	pgs = append(pgs, projected.ProbeVolumePlugins()...)
	pgs = append(pgs, portworx.ProbeVolumePlugins()...)
	pgs = append(pgs, scaleio.ProbeVolumePlugins()...)
	pgs = append(pgs, local.ProbeVolumePlugins()...)
	pgs = append(pgs, storageos.ProbeVolumePlugins()...)

	// TODO: not working in local, not working in remote as well
	// E0524 | csi_plugin.go:271] Failed to initialize CSINodeInfo: error updating CSINode annotation: timed out waiting for the condition; caused by: the server could not find the requested resource
	// F0524 | 20838 csi_plugin.go:285] Failed to initialize CSINodeInfo after retrying
	//
	// for remote nodes, make sure to update role
	// E0525 | csi_plugin.go:271] Failed to initialize CSINodeInfo: error updating CSINode annotation: timed out waiting for the condition; caused by: csinodes.storage.k8s.io "hollowwandefortegreen6wd8z" is forbidden: User "system:serviceaccount:eks-2020052423-boldlyuxvugd-hollow-nodes-remote:hollow-nodes-remote-service-account" cannot get resource "csinodes" in API group "storage.k8s.io" at the cluster scope
	// F0525 | csi_plugin.go:285] Failed to initialize CSINodeInfo after retrying
	//
	// "k8s.io/kubernetes/pkg/volume/csi"
	// pgs = append(pgs, csi.ProbeVolumePlugins()...)

	return pgs
}
