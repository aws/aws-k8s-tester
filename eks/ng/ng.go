// Package ng implements EKS worker nodes with a custom AMI.
package ng

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines Node Group configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	IAMAPI    iamiface.IAMAPI
	CFNAPI    cloudformationiface.CloudFormationAPI
	EC2API    ec2iface.EC2API
	ASGAPI    autoscalingiface.AutoScalingAPI
	EKSAPI    eksiface.EKSAPI
	SSMAPI    ssmiface.SSMAPI
	S3API     s3iface.S3API
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester implements EKS "Node Group" for "kubetest2" Deployer.
// ref. https://github.com/kubernetes/test-infra/blob/master/kubetest2/pkg/types/types.go
// ref. https://docs.aws.amazon.com/eks/latest/userguide/create-managed-node-group.html
// ref. https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-eks-nodegroup.html
type Tester interface {
	// Create creates EKS "Node Group", and waits for completion.
	Create() error
	// Delete deletes all EKS "Node Group" resources.
	Delete() error

	// FetchLogs fetches logs from all worker nodes.
	FetchLogs() error
	// DownloadClusterLogs dumps all logs to artifact directory.
	// Let default kubetest log dumper handle all artifact uploads.
	// See https://github.com/kubernetes/test-infra/pull/9811/files#r225776067.
	DownloadClusterLogs(artifactDir string) error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{
		cfg:    cfg,
		logsMu: new(sync.RWMutex),
	}, nil
}

type tester struct {
	cfg        Config
	logsMu     *sync.RWMutex
	failedOnce bool
}

func (ts *tester) Create() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		ts.cfg.Logger.Info("node group is disabled; skipping creation")
		return nil
	}
	if ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("node group is already created; skipping creation")
		return nil
	}
	if len(ts.cfg.EKSConfig.Parameters.PublicSubnetIDs) == 0 {
		return errors.New("empty EKSConfig.Parameters.PublicSubnetIDs")
	}

	defer func() {
		ts.cfg.EKSConfig.AddOnNodeGroups.Created = true
		ts.cfg.EKSConfig.Sync()
	}()

	if err = ts.createRole(); err != nil {
		return err
	}
	if err = ts.createSG(); err != nil {
		return err
	}
	if err = ts.createASGs(); err != nil {
		return err
	}
	if err = ts.createConfigMap(); err != nil {
		return err
	}
	if err = ts.waitForNodes(); err != nil {
		return err
	}
	if err = ts.createSSM(); err != nil {
		return err
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitForNodes() error {
	waitDur := 5 * time.Minute
	var items []v1.Node
	ts.cfg.Logger.Info("checking nodes via client-go")
	for _, mv := range ts.cfg.EKSConfig.AddOnNodeGroups.ASGs {
		retryStart, threshold := time.Now(), 3*time.Minute
		for time.Now().Sub(retryStart) < waitDur {
			select {
			case <-ts.cfg.Stopc:
				return errors.New("checking nodes aborted")
			case <-ts.cfg.Sig:
				return errors.New("checking nodes aborted")
			case <-time.After(5 * time.Second):
			}

			nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(metav1.ListOptions{})
			if err != nil {
				ts.cfg.Logger.Error("get nodes failed", zap.Error(err))
				continue
			}
			items = nodes.Items

			readies := int64(0)
			for _, node := range items {
				for _, cond := range node.Status.Conditions {
					if cond.Type != v1.NodeReady {
						continue
					}
					ts.cfg.Logger.Info("node info",
						zap.String("name", node.GetName()),
						zap.String("type", fmt.Sprintf("%s", cond.Type)),
						zap.String("status", fmt.Sprintf("%s", cond.Status)),
					)
					if cond.Status == v1.ConditionTrue {
						readies++
					}
				}
			}
			ts.cfg.Logger.Info("nodes",
				zap.Int64("current-ready-nodes", readies),
				zap.Int64("desired-ready-nodes", mv.ASGDesiredCapacity),
			)
			if readies >= mv.ASGDesiredCapacity {
				break
			}
			took := time.Now().Sub(retryStart)
			if took > threshold {
				fmt.Printf("\n\nkubectl (%q, %q)\n\n", ts.cfg.EKSConfig.ConfigPath, ts.cfg.EKSConfig.KubectlCommand())
				fmt.Println(ts.cfg.EKSConfig.KubectlCommands())
				fmt.Printf("\n\nSSH (%q, %q)\n\n", ts.cfg.EKSConfig.ConfigPath, ts.cfg.EKSConfig.KubectlCommand())
				fmt.Println(ts.cfg.EKSConfig.SSHCommands())
			}
		}
	}
	println()
	for _, v := range items {
		fmt.Printf("'Node' %q (using client-go): %+v\n", v.GetName(), v.Status.Addresses)
	}
	println()

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.IsEnabledAddOnNodeGroups() {
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnNodeGroups.Created {
		ts.cfg.Logger.Info("ManagedNodeGroup is not created; skipping deletion")
		return nil
	}

	var errs []string
	if err := ts.deleteSSM(); err != nil {
		ts.cfg.Logger.Warn("failed to delete SSM", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteASGs(); err != nil {
		ts.cfg.Logger.Warn("failed to delete ASGs", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteSG(); err != nil {
		ts.cfg.Logger.Warn("failed to delete SG", zap.Error(err))
		errs = append(errs, err.Error())
	}
	if err := ts.deleteRole(); err != nil {
		ts.cfg.Logger.Warn("failed to delete role", zap.Error(err))
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.EKSConfig.Sync()
}
