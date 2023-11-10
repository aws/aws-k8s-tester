package eksctl

import (
	"fmt"
	"os"

	"github.com/aws/aws-k8s-tester/kubetest2/internal/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"k8s.io/klog"
)

type UpOptions struct {
	Region            string   `flag:"region" desc:"AWS region for EKS cluster"`
	KubernetesVersion string   `flag:"kubernetes-version" desc:"cluster Kubernetes version"`
	Nodes             int      `flag:"nodes" desc:"number of nodes to launch in cluster"`
	AMI               string   `flag:"ami" desc:"Node AMI"`
	InstanceTypes     []string `flag:"instance-types" desc:"Node instance types"`
}

func (d *deployer) verifyUpFlags() error {
	if d.KubernetesVersion == "" {
		klog.Infof("--kubernetes-version is empty, attempting to detect it...")
		detectedVersion, err := detectKubernetesVersion()
		if err != nil {
			return fmt.Errorf("unable to detect --kubernetes-version, flag cannot be empty")
		}
		klog.Infof("detected --kubernetes-version=%s", detectedVersion)
		d.KubernetesVersion = detectedVersion
	}
	if d.Nodes < 0 {
		return fmt.Errorf("number of nodes must be greater than zero")
	}
	if d.Nodes == 0 {
		d.Nodes = 4
		klog.V(2).Infof("Using default number of nodes: %d", d.Nodes)
	}
	return nil
}

func (d *deployer) Up() error {
	if err := d.verifyUpFlags(); err != nil {
		return fmt.Errorf("up flags are invalid: %v", err)
	}
	klog.Infof("creating cluster: %s", d.commonOptions.RunID())
	kubeconfig, err := d.Kubeconfig()
	if err != nil {
		return err
	}
	clusterConfig, err := d.RenderClusterConfig()
	if err != nil {
		return err
	}
	klog.Infof("rendered cluster config: %s", string(clusterConfig))
	clusterConfigFile, err := os.CreateTemp("", "kubetest2-eksctl-cluster-config")
	if err != nil {
		return err
	}
	defer clusterConfigFile.Close()
	_, err = clusterConfigFile.Write(clusterConfig)
	if err != nil {
		return err
	}
	args := []string{
		"create",
		"cluster",
		"--install-nvidia-plugin=false",
		"--install-neuron-plugin=false",
		"--config-file", clusterConfigFile.Name(),
		"--kubeconfig", kubeconfig,
	}
	err = util.ExecuteCommand("eksctl", args...)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %v", err)
	}
	return nil
}

func (d *deployer) IsUp() (up bool, err error) {
	result, err := d.eksClient.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(d.commonOptions.RunID()),
	})
	if err != nil {
		return false, err
	}
	status := aws.StringValue(result.Cluster.Status)
	switch status {
	case eks.ClusterStatusActive:
		return true, nil
	case eks.ClusterStatusCreating:
		return false, nil
	default:
		return false, fmt.Errorf("cluster status is: %v", status)
	}
}

func detectKubernetesVersion() (string, error) {
	detectedVersion, err := util.DetectKubernetesVersion()
	if err != nil {
		return "", err
	}
	minorVersion, err := util.ParseMinorVersion(detectedVersion)
	if err != nil {
		return "", err
	}
	return minorVersion, nil
}
