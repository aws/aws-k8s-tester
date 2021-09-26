package cni

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Stopc     chan struct{} `json:"-"`
	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum_nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
	// CNINamespace is the namespace the CNI daemonset is deployed to
	CNINamespace string `json:"cni_namespace"`
}

func (cfg *Config) ValidateAndSetDefaults() error {
	if cfg.MinimumNodes == 0 {
		cfg.MinimumNodes = DefaultMinimumNodes
	}
	if cfg.Namespace == "" {
		return errors.New("empty Namespace")
	}
	if cfg.CNINamespace == "" {
		cfg.CNINamespace = DefaultCNINamespace
	}
	return nil
}

const (
	DefaultCNINamespace string = "kube-system"
	DefaultMinimumNodes int    = 2
	ServerPod           string = "cni-server-pod"
	PingPod             string = "cni-ping-pod"
	NodePod             string = "cni-node-pod"
	PodTimeout                 = 2 * time.Minute
)

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       true,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

var graceperiod = int64(0)

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func Env() string {
	return "ADD_ON_" + strings.ToUpper(strings.Replace(pkgName, "-", "_", -1))
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}
	if nodes, err := client.ListNodes(ts.cfg.Client.KubernetesClient()); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}
	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), ts.cfg.Namespace); err != nil {
		return err
	}
	if err := ts.checkCNI(); err != nil {
		return err
	}
	if err := ts.testPodtoPod(); err != nil {
		return err
	}
	if err := ts.testPodtoNode(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	var errs []string
	if err := ts.deletePodtoPod(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to Pod to Pod Server CNI tester (%v)", err))
	}
	if err := ts.deletePodtoNode(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to Pod to Node Server CNI tester (%v)", err))
	}
	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

func (ts *tester) checkCNI() error {
	// List the pods in the namespace
	daemonsetlist, err := client.ListDaemonSets(
		ts.cfg.Logger,
		ts.cfg.Client.KubernetesClient(),
		ts.cfg.CNINamespace,
		5,
		5*time.Second,
	)
	if err != nil {
		ts.cfg.Logger.Warn("'daemonset list' failed", zap.Error(err))
	}
	for _, daemonset := range daemonsetlist {
		switch daemonset.ObjectMeta.Name {
		case "aws-node":
			ts.cfg.Logger.Info("Using CNI:", zap.String("CNI", daemonset.ObjectMeta.Name))

		case "cillium":
			ts.cfg.Logger.Info("Using CNI:", zap.String("CNI", daemonset.ObjectMeta.Name))
		}
	}
	return nil
}

func (ts *tester) testPodtoPod() error {
	//Create Server Pod
	ts.cfg.Logger.Info("Creating ServerPod:", zap.String("ServerPod", ServerPod))
	serverPod := client.NewBusyBoxPod(ServerPod, "sleep 120")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	serverPod, err := ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Create(ctx, serverPod, meta_v1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create CNI server pod (%v)", err)
	}
	cancel()
	ts.cfg.Logger.Info("Checking for ServerPod completed", zap.String("ServerPod", ServerPod))
	err = client.WaitForPodRunningInNamespace(ts.cfg.Client.KubernetesClient(), serverPod)
	if err != nil {
		return fmt.Errorf("failed to wait for CNI server pod to become healthy (%v)", err)
	}
	serverPod, err = ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Get(context.TODO(), ServerPod, meta_v1.GetOptions{})
	//Create Ping Pod
	ts.cfg.Logger.Info("Creating PingPod:", zap.String("PingPod", PingPod))
	pingPod := client.NewBusyBoxPod(PingPod, "ping -c 3 -w 2 -w 30 "+serverPod.Status.PodIP)
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	pingPod, err = ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Create(ctx, pingPod, meta_v1.CreateOptions{})
	cancel()
	ts.cfg.Logger.Info("Checking for PingPod completed", zap.String("ServerPod", ServerPod))
	err = client.WaitForPodRunningInNamespace(ts.cfg.Client.KubernetesClient(), pingPod)
	if err != nil {
		return fmt.Errorf("failed to wait for CNI Ping pod to become healthy (%v)", err)
	}
	//Check for Ping Pod Success
	err = client.WaitForPodSuccessInNamespaceTimeout(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), pingPod.Name, ts.cfg.Namespace, PodTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for CNI Ping pod to become healthy (%v)", err)
	}
	ts.cfg.Logger.Info("Pod to Pod communication SUCCESS")
	return nil
}

func (ts *tester) deletePodtoPod() error {
	//Delete Server Pod
	ts.cfg.Logger.Info("Deleting ServerPod", zap.String("ServerPod", ServerPod))
	foreground := meta_v1.DeletePropagationForeground
	err := ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Delete(context.TODO(), ServerPod,
		meta_v1.DeleteOptions{
			GracePeriodSeconds: int64Ref(0),
			PropagationPolicy:  &foreground,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to Delete CNI Server pod (%v)", err)
	}
	return nil
}

func (ts *tester) testPodtoNode() error {
	//Find random schedule-able node and it's IP
	node, err := client.GetRandomReadySchedulableNode(ts.cfg.Client.KubernetesClient())
	if err != nil {
		return fmt.Errorf("failed getting random ready schedulable node (%v)", err)
	}
	internalIP, err := client.GetInternalIP(node)
	ts.cfg.Logger.Info("Found Random schedulabe Node:", zap.String("Node IP", internalIP))
	if err != nil {
		return fmt.Errorf("failed IP of schedulable node (%v)", err)
	}
	//Create Node Pod
	ts.cfg.Logger.Info("Creating NodePod:", zap.String("NodePod", NodePod))
	nodePod := client.NewBusyBoxPod(NodePod, "ping -c 3 -w 2 -w 30 "+internalIP)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	nodePod, err = ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Create(ctx, nodePod, meta_v1.CreateOptions{})
	cancel()
	ts.cfg.Logger.Info("Checking for NodePod completed", zap.String("NodePod", NodePod))
	err = client.WaitForPodRunningInNamespace(ts.cfg.Client.KubernetesClient(), nodePod)
	if err != nil {
		return fmt.Errorf("failed to wait for CNI Node pod to become healthy (%v)", err)
	}
	//Check for Node Pod Success
	ts.cfg.Logger.Info("Checking for NodePod success")
	err = client.WaitForPodSuccessInNamespaceTimeout(ts.cfg.Logger, ts.cfg.Client.KubernetesClient(), nodePod.Name, ts.cfg.Namespace, PodTimeout)
	if err != nil {
		return fmt.Errorf("failed to wait for CNI Node pod to become healthy (%v)", err)
	}
	ts.cfg.Logger.Info("Pod to Node communication SUCCESS")
	return nil
}

func (ts *tester) deletePodtoNode() error {
	//Delete Server Pod
	foreground := meta_v1.DeletePropagationForeground
	ts.cfg.Logger.Info("Deleting NodePod", zap.String("NodePod", NodePod))
	err := ts.cfg.Client.KubernetesClient().CoreV1().Pods(ts.cfg.Namespace).Delete(context.TODO(), NodePod,
		meta_v1.DeleteOptions{
			GracePeriodSeconds: int64Ref(0),
			PropagationPolicy:  &foreground,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to Delete CNI Node pod (%v)", err)
	}
	return nil
}

func int64Ref(v int64) *int64 {
	return &v
}
