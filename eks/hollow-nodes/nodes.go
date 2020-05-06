package hollownodes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
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
	Client         k8s_client.EKS
	Stopc          chan struct{}
	Nodes          int
	NodeLabels     map[string]string
	MaxOpenFiles   int64
	KubectlPath    string
	KubeConfigPath string
}

type nodeGroup struct {
	mu sync.Mutex

	cfg NodeGroupConfig

	donec          chan struct{}
	donecCloseOnce *sync.Once

	kubelets []*hollowKubelet
}

// CreateNodeGroup creates a new hollow node group.
func CreateNodeGroup(cfg NodeGroupConfig) (ng NodeGroup, err error) {
	if !fileutil.Exist(cfg.KubectlPath) {
		return nil, fmt.Errorf("KubectlPath does not exist [%q]", cfg.KubectlPath)
	}
	if !fileutil.Exist(cfg.KubeConfigPath) {
		return nil, fmt.Errorf("KubeConfigPath does not exist [%q]", cfg.KubeConfigPath)
	}
	return &nodeGroup{
		cfg:            cfg,
		donec:          make(chan struct{}),
		donecCloseOnce: new(sync.Once),
	}, nil
}

func (ng *nodeGroup) Start() (err error) {
	ng.mu.Lock()
	defer ng.mu.Unlock()

	ng.kubelets = make([]*hollowKubelet, ng.cfg.Nodes)

	ng.cfg.Logger.Info("creating node group with hollow nodes", zap.Int("nodes", ng.cfg.Nodes), zap.Any("node-labels", ng.cfg.NodeLabels))
	for i := 0; i < ng.cfg.Nodes; i++ {
		nodeName := fmt.Sprintf("fake-node-%06d-%s", i, randutil.String(5))
		ng.kubelets[i], err = newKubelet(kubeletConfig{
			lg:           ng.cfg.Logger,
			cli:          ng.cfg.Client,
			stopc:        ng.cfg.Stopc,
			donec:        ng.donec,
			nodeName:     nodeName,
			nodeLabels:   ng.cfg.NodeLabels,
			maxOpenFiles: ng.cfg.MaxOpenFiles,
		})
		if err != nil {
			return err
		}
	}
	ng.cfg.Logger.Info("created node group with hollow nodes", zap.Int("nodes", ng.cfg.Nodes))

	ng.cfg.Logger.Info("starting hollow node group")
	for _, node := range ng.kubelets {
		go node.run()
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
		node.stop()
	}
	ng.cfg.Logger.Info("stopped hollow node group")
}

func (ng *nodeGroup) CheckNodes() (readyNodes []string, createdNodes []string, err error) {
	ng.mu.Lock()
	defer ng.mu.Unlock()
	return ng.checkNodes()
}

func (ng *nodeGroup) checkNodes() (readyNodes []string, createdNodes []string, err error) {
	argsGetCSRs := []string{
		ng.cfg.KubectlPath,
		"--kubeconfig=" + ng.cfg.KubeConfigPath,
		"get",
		"csr",
		"-o=wide",
	}
	cmdGetCSRs := strings.Join(argsGetCSRs, " ")

	argsGetNodes := []string{
		ng.cfg.KubectlPath,
		"--kubeconfig=" + ng.cfg.KubeConfigPath,
		"get",
		"nodes",
		"--show-labels",
		"-o=wide",
	}
	cmdGetNodes := strings.Join(argsGetNodes, " ")

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
				ng.cfg.Logger.Warn("unexpected node labels", zap.String("node-name", nodeName), zap.Any("expected", ng.cfg.NodeLabels), zap.Any("got", labels))
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

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsGetCSRs[0], argsGetCSRs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ng.cfg.Logger.Warn("'kubectl get csr' failed", zap.Error(err))
		}
		fmt.Printf("\n\n\"%s\":\n%s\n", cmdGetCSRs, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(ctx, argsGetNodes[0], argsGetNodes[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ng.cfg.Logger.Warn("'kubectl get nodes' failed", zap.Error(err))
		}
		fmt.Printf("\n\"%s\":\n%s\n", cmdGetNodes, out)

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
