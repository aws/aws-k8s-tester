package clusterloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func Test_installClusterloader(t *testing.T) {
	t.Skip()

	err := installClusterloader(zap.NewExample(), DefaultClusterloaderPath(), DefaultClusterloaderDownloadURL())
	if err != nil {
		t.Skip(err)
	}
}

func Test_parsePodStartupLatency(t *testing.T) {
	perfDatas := []PerfData{}
	err := filepath.Walk("test-data", func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), "PodStartupLatency-") {
			return nil
		}
		p, perr := parsePodStartupLatency(path)
		if perr != nil {
			return perr
		}
		perfDatas = append(perfDatas, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(perfDatas) != 5 {
		t.Fatalf("expected 5 data, got %d", len(perfDatas))
	}
	for _, v := range perfDatas {
		fmt.Println(v)
	}

	fmt.Printf("%+v\n", mergePodStartupLatency(perfDatas...))
}
