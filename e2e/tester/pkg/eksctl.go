package pkg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type EksctlClusterCreator struct {
	// TestId is used as cluster name
	TestId string

	// EksClsuter represents the configuration to create EKS cluster
	EksCluster *EksCluster

	// directory where tempory data is saved
	TestDir string

	// The path to eksctl executable
	EksctlBinaryPath string
}

func NewEksctlClusterCreator(ekscluster *EksCluster, dir string, testId string) *EksctlClusterCreator {
	binaryFilePath := filepath.Join(dir, "eksctl")
	return &EksctlClusterCreator{
		EksCluster:       ekscluster,
		TestDir:          dir,
		TestId:           testId,
		EksctlBinaryPath: binaryFilePath,
	}
}

func (c *EksctlClusterCreator) Init() (Step, error) {
	f := func(ctx context.Context) error {
		_, err := os.Stat(c.TestDir)
		if os.IsNotExist(err) {
			err := os.Mkdir(c.TestDir, 0777)
			if err != nil {
				return err
			}
		}

		_, err = os.Stat(c.EksctlBinaryPath)
		if os.IsNotExist(err) {
			return c.downloadEksctl()
		}

		return nil
	}

	return &FuncStep{f}, nil

}

func (c *EksctlClusterCreator) Up() (Step, error) {
	fmt.Println(c.EksCluster)

	f := func(ctx context.Context) error {
		err := c.createCluster(ctx)
		if err != nil {
			return err
		}
		return nil
	}
	return &FuncStep{f}, nil
}

func (c *EksctlClusterCreator) TearDown() (Step, error) {
	f := func(ctx context.Context) error {
		clusterName := c.clusterName()
		log.Printf("Deleting cluster %s", clusterName)

		cmd := exec.CommandContext(ctx, c.EksctlBinaryPath, "delete", "cluster",
			"--name", clusterName)

		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr

		return cmd.Run()
	}

	return &FuncStep{f}, nil
}

func (c *EksctlClusterCreator) clusterName() string {
	return fmt.Sprintf("test-eks-cluster-%s", c.TestId)
}

func (c *EksctlClusterCreator) downloadEksctl() error {
	var osArch string
	switch runtime.GOOS {
	case "linux":
		osArch = "Linux_amd64"
	case "windows":
		osArch = "Windows_amd64"
	case "darwin":
		osArch = "Darwin_amd64"
	default:
		return fmt.Errorf("GOOS %s is not supported", runtime.GOOS)
	}

	url := fmt.Sprintf("https://github.com/weaveworks/eksctl/releases/download/0.6.0/eksctl_%s.tar.gz", osArch)
	log.Printf("Downloading eksctl from %s to %s", url, c.EksctlBinaryPath)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = untar(c.TestDir, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)
		fmt.Println(target)

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

			// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func (c *EksctlClusterCreator) createCluster(ctx context.Context) error {
	clusterName := c.clusterName()
	log.Printf("Creating EKS cluster %s", clusterName)

	cmd := exec.CommandContext(ctx, c.EksctlBinaryPath, "create", "cluster",
		"--name", clusterName,
		"--region", c.EksCluster.Region,
		"--version", c.EksCluster.KubernetesVersion,
		"--nodes", fmt.Sprintf("%d", c.EksCluster.NodeCount),
		"--node-type", c.EksCluster.NodeSize,
	)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
