package pkg

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type KopsClusterCreator struct {
	// TestId is used as cluster name
	TestId string

	// Kops represents the configuration to create kops cluster
	Kops *KopsCluster

	// directory where tempory data is saved
	TestDir string

	// The path to Kops executable
	KopsBinaryPath string
}

func NewKopsClusterCreator(kops *KopsCluster, dir string, testId string) *KopsClusterCreator {
	binaryFilePath := filepath.Join(dir, "kops")
	return &KopsClusterCreator{
		Kops:           kops,
		TestDir:        dir,
		TestId:         testId,
		KopsBinaryPath: binaryFilePath,
	}
}

func (c *KopsClusterCreator) Init() (Step, error) {
	f := func() error {
		_, err := os.Stat(c.TestDir)
		if os.IsNotExist(err) {
			err := os.Mkdir(c.TestDir, 0777)
			if err != nil {
				return err
			}
		}

		_, err = os.Stat(c.KopsBinaryPath)
		if os.IsNotExist(err) {
			return c.downloadKops()
		}

		return nil
	}

	return &FuncStep{f}, nil
}

func (c *KopsClusterCreator) Up() (Step, error) {
	f := func() error {
		// create cluster
		err := c.createCluster()
		if err != nil {
			return err
		}

		// wait for cluster creation to success
		// or return err if timedout
		err = c.waitForCreation(15 * time.Minute)
		if err != nil {
			return err
		}

		return nil
	}

	return &FuncStep{f}, nil
}

func (c *KopsClusterCreator) TearDown() (Step, error) {
	f := func() error {
		clusterName := c.clusterName()
		log.Printf("Deleting cluster %s", clusterName)

		cmd := exec.Command(c.KopsBinaryPath, "delete", "cluster",
			"--state", c.Kops.StateFile,
			"--name", clusterName, "--yes")

		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	return &FuncStep{f}, nil
}

func (c *KopsClusterCreator) clusterName() string {
	return fmt.Sprintf("test-cluster-%s.k8s.local", c.TestId)
}

func (c *KopsClusterCreator) downloadKops() error {
	osArch := fmt.Sprintf("%s-amd64", runtime.GOOS)
	url := fmt.Sprintf("https://github.com/kubernetes/kops/releases/download/1.14.0-alpha.1/kops-%s", osArch)
	log.Printf("Downloading KOPS from %s to %s", url, c.KopsBinaryPath)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(c.KopsBinaryPath, payload, 0777)
	if err != nil {
		return err
	}

	return nil
}

func (c *KopsClusterCreator) createCluster() error {
	clusterName := c.clusterName()
	log.Printf("Creating Kops cluster %s", clusterName)

	sshKeyPath := filepath.Join(c.TestDir, "id_rsa")

	_, err := os.Stat(sshKeyPath)
	// only generate SSH key if it is missing
	if os.IsNotExist(err) {
		err := c.generateSSHKey(sshKeyPath)
		if err != nil {
			return err
		}
	}

	cmd := exec.Command(c.KopsBinaryPath, "create", "cluster",
		"--state", c.Kops.StateFile,
		"--zones", c.Kops.Zones,
		"--node-count", fmt.Sprintf("%d", c.Kops.NodeCount),
		"--node-size", c.Kops.NodeSize,
		"--kubernetes-version", c.Kops.KubernetesVersion,
		"--ssh-public-key", fmt.Sprintf("%s.pub", sshKeyPath),
		clusterName,
	)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}
	clusterYamlPath := filepath.Join(c.TestDir, fmt.Sprintf("%s.yaml", c.TestId))

	clusterYamlFile, err := os.Create(clusterYamlPath)
	if err != nil {
		return err
	}
	defer clusterYamlFile.Close()

	cmd = exec.Command(c.KopsBinaryPath, "get", "cluster",
		"--state", c.Kops.StateFile, clusterName, "-o", "yaml")
	cmd.Stdout = clusterYamlFile
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return err
	}

	_, err = clusterYamlFile.WriteString(c.Kops.FeatureGates)
	if err != nil {
		return err
	}
	_, err = clusterYamlFile.WriteString(c.Kops.IamPolicies)
	if err != nil {
		return err
	}

	cmd = exec.Command(c.KopsBinaryPath, "replace",
		"--state", c.Kops.StateFile, "-f", clusterYamlPath)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command(c.KopsBinaryPath, "update", "cluster",
		"--state", c.Kops.StateFile, clusterName, "--yes")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

// waitForCreatoin waits for cluster creation and times out if it takes too long
func (c *KopsClusterCreator) waitForCreation(timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("cluster is not created after %v", timeout)
		default:
			cmd := exec.Command(c.KopsBinaryPath, "validate", "cluster",
				"--state", c.Kops.StateFile)
			cmd.Stdout = os.Stdout
			cmd.Stdin = os.Stdin
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err == nil {
				timer.Stop()
				return nil
			}
			time.Sleep(30 * time.Second)
		}
	}
}

func (c *KopsClusterCreator) generateSSHKey(keyPath string) error {
	cmd := exec.Command("ssh-keygen", "-N", "", "-f", keyPath)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
