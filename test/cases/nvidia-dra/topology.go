package nvidia_dra

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/aws/aws-k8s-tester/test/common"
)

//go:embed templates/nccl-test-mpijob.yaml.tmpl
var mpijobTemplate string

// ---------------------------------------------------------------------------
// Instance topology
// ---------------------------------------------------------------------------

// NvidiaInstanceTopology describes the GPU/EFA hardware topology for an NVIDIA instance family.
type NvidiaInstanceTopology struct {
	Family         string
	GPUsPerNode    int    // total GPUs per node (e.g. 8 for p5.48xlarge)
	AllGPUCount    int    // same as GPUsPerNode for "All" allocation mode
	RdmaType       string // RDMA device type (e.g. "efa")
	RCTSubDir      string // subdirectory under rcts/
	TestCaseSubDir string // subdirectory under testcases/
}

var instanceTopologies = map[string]NvidiaInstanceTopology{
	"p5": {
		Family:         "p5",
		GPUsPerNode:    8,
		AllGPUCount:    8,
		RdmaType:       "efa",
		RCTSubDir:      "p5",
		TestCaseSubDir: "p5",
	},
}

// GetTopologyForNodeType returns the NvidiaInstanceTopology for a given node type
// (e.g. "p5.48xlarge"). It extracts the family prefix before the first "."
// and looks it up in the registry.
func GetTopologyForNodeType(nodeType string) (*NvidiaInstanceTopology, error) {
	family := common.ExtractFamily(nodeType)
	topo, ok := instanceTopologies[family]
	if !ok {
		return nil, fmt.Errorf("unsupported instance family %q (from %q); supported: %s",
			family, nodeType, supportedFamilies())
	}
	return &topo, nil
}

func supportedFamilies() string {
	families := make([]string, 0, len(instanceTopologies))
	for k := range instanceTopologies {
		families = append(families, k)
	}
	return strings.Join(families, ", ")
}

// ---------------------------------------------------------------------------
// MPIJob rendering
// ---------------------------------------------------------------------------

// NvidiaMPIJobParams holds all template parameters for rendering the NCCL MPIJob YAML.
type NvidiaMPIJobParams struct {
	SlotsPerWorker     int
	TotalProcesses     int
	WorkerReplicas     int
	ContainerTestImage string
	ResourceClaims     []common.ResourceClaimRef
}

// RenderNvidiaMPIJobYAML renders the embedded NCCL MPIJob Go template with the given params
// and returns the resulting YAML bytes.
func RenderNvidiaMPIJobYAML(params NvidiaMPIJobParams) ([]byte, error) {
	tmpl, err := template.New("mpijob").Parse(mpijobTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing MPIJob template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("rendering MPIJob template: %w", err)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// NVIDIA-specific helpers
// ---------------------------------------------------------------------------

// getGPUCount returns the GPU device count from an RCT.
// For AllocationMode "All" it returns the topology's AllGPUCount;
// otherwise it returns the explicit Count from the gpu.nvidia.com request.
func getGPUCount(rct *common.ResourceClaimTemplateSpec, topo *NvidiaInstanceTopology) int {
	for _, req := range rct.Spec.Spec.Devices.Requests {
		if req.DeviceClassName != "gpu.nvidia.com" {
			continue
		}
		if req.AllocationMode == "All" {
			return topo.AllGPUCount
		}
		if req.Count <= 0 {
			log.Printf("[WARN] gpu.nvidia.com request has non-positive count: %d", req.Count)
		}
		return req.Count
	}
	log.Printf("[WARN] no gpu.nvidia.com device request found in RCT, returning GPU count 0")
	return 0
}

// ComputeNvidiaMPIJobParams computes MPIJob parameters from a test case spec.
// It resolves each claim's resourceClaimTemplateName against the RCT index to
// get the GPU count, then calculates SlotsPerWorker and TotalProcesses.
func ComputeNvidiaMPIJobParams(tc *common.TestCaseSpec, rctIndex map[string]*common.ResourceClaimTemplateSpec, topo *NvidiaInstanceTopology, workerReplicas int, containerTestImage string) (*NvidiaMPIJobParams, error) {
	if topo == nil {
		return nil, fmt.Errorf("instance topology is required")
	}
	if workerReplicas <= 0 {
		return nil, fmt.Errorf("workerReplicas must be positive, got %d", workerReplicas)
	}
	if containerTestImage == "" {
		return nil, fmt.Errorf("containerTestImage is required")
	}

	totalGPUs := 0
	var claims []common.ResourceClaimRef

	for _, tcClaim := range tc.ResourceClaims {
		rct, ok := rctIndex[tcClaim.ResourceClaimTemplateName]
		if !ok {
			return nil, fmt.Errorf("resource claim template %q not found in RCT index", tcClaim.ResourceClaimTemplateName)
		}

		totalGPUs += getGPUCount(rct, topo)

		claims = append(claims, common.ResourceClaimRef{
			Name:         tcClaim.Name,
			TemplateName: tcClaim.ResourceClaimTemplateName,
		})
	}

	slotsPerWorker := totalGPUs
	totalProcesses := slotsPerWorker * workerReplicas

	return &NvidiaMPIJobParams{
		SlotsPerWorker:     slotsPerWorker,
		TotalProcesses:     totalProcesses,
		WorkerReplicas:     workerReplicas,
		ContainerTestImage: containerTestImage,
		ResourceClaims:     claims,
	}, nil
}
