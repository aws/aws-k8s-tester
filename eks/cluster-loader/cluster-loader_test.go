package clusterloader

import (
	"context"
	"fmt"
	"os"
	os_exec "os/exec"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func TestExec(t *testing.T) {
	kubectlPath, err := os_exec.LookPath("kubectl")
	if err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	var output []byte
	output, err = exec.New().CommandContext(
		ctx,
		kubectlPath,
		"--help",
	).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	fmt.Println(out, err)

	loaderPath := "/tmp/clusterloader-test"
	defer os.RemoveAll(loaderPath)
	ld := &loader{
		cfg: Config{
			Logger:    zap.NewExample(),
			LogWriter: os.Stderr,

			ClusterLoaderPath:        loaderPath,
			ClusterLoaderDownloadURL: "https://github.com/aws/aws-k8s-tester/releases/download/v1.4.6/clusterloader2-linux-amd64",

			PodsPerNode: 100,
		},
	}
	// if err = ld.downloadClusterLoader(); err != nil {
	// 	t.Fatal(err)
	// }

	err = ld.writeTestOverrides()
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(ld.testOverridesPath)
}
