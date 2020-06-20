package clusterloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	measurement_util "k8s.io/perf-tests/clusterloader2/pkg/measurement/util"
)

func TestParsePodStartupLatency(t *testing.T) {
	perfDatas := []measurement_util.PerfData{}
	err := filepath.Walk("artifacts", func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasPrefix(filepath.Base(path), "PodStartupLatency_") {
			return nil
		}
		p, perr := ParsePodStartupLatency(path)
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

	fmt.Printf("%+v\n", MergePodStartupLatency(perfDatas...))
}
