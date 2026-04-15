//go:build e2e

package nvidia_dra

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/aws/aws-k8s-tester/test/common"
)

//go:embed testcases
var embeddedTestCases embed.FS

func TestNvidiaDRAMultiNode(t *testing.T) {
	topo, err := GetTopologyForNodeType(*nodeType)
	if err != nil {
		t.Fatalf("resolving topology for %s: %v", *nodeType, err)
	}

	rctDir := filepath.Join("rcts", topo.RCTSubDir)
	rctIndex, err := common.LoadRCTIndex(rctsFS, rctDir)
	if err != nil {
		t.Fatalf("loading RCT index from %s: %v", rctDir, err)
	}

	tcDir := filepath.Join("testcases", topo.TestCaseSubDir)

	featureList, err := common.DiscoverAndBuildFeatures(
		embeddedTestCases,
		tcDir,
		rctIndex,
		"nvidia-dra",
		"multi-node-nccl-test",
		nodeCount,
		func(tc *common.TestCaseSpec, rctIndex map[string]*common.ResourceClaimTemplateSpec) ([]byte, error) {
			params, err := ComputeNvidiaMPIJobParams(tc, rctIndex, topo, nodeCount, *containerTestImage)
			if err != nil {
				return nil, err
			}
			return RenderNvidiaMPIJobYAML(*params)
		},
		clientset,
	)
	if err != nil {
		t.Fatalf("discovering and building features: %v", err)
	}

	if len(featureList) == 0 {
		t.Logf("no test cases found under %s, skipping", tcDir)
		return
	}

	testenv.Test(t, featureList...)
}
