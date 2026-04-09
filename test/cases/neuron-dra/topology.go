package neuron_dra

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

//go:embed templates/nccom-test-mpijob.yaml.tmpl
var mpijobTemplate string

// ---------------------------------------------------------------------------
// Instance topology
// ---------------------------------------------------------------------------

// InstanceTopology describes the Neuron/EFA hardware topology for an instance family.
type InstanceTopology struct {
	Family               string
	NeuronCoresPerDevice int
	AllNeuronCount       int
	RdmaType             string // RDMA device type (e.g. "efa")
	RCTSubDir            string // subdirectory under rcts/
	TestCaseSubDir       string // subdirectory under testcases/
}

var instanceTopologies = map[string]InstanceTopology{
	"trn1": {
		Family:               "trn1",
		NeuronCoresPerDevice: 2,
		AllNeuronCount:       16,
		RdmaType:             "efa",
		RCTSubDir:            "trn1",
		TestCaseSubDir:       "trn1",
	},
}

// GetTopologyForNodeType returns the InstanceTopology for a given node type
// (e.g. "trn1.32xlarge"). It extracts the family prefix before the first "."
// and looks it up in the registry.
func GetTopologyForNodeType(nodeType string) (*InstanceTopology, error) {
	family := extractFamily(nodeType)
	topo, ok := instanceTopologies[family]
	if !ok {
		return nil, fmt.Errorf("unsupported instance family %q (from %q); supported: %s",
			family, nodeType, supportedFamilies())
	}
	return &topo, nil
}

func extractFamily(nodeType string) string {
	if idx := strings.Index(nodeType, "."); idx > 0 {
		return nodeType[:idx]
	}
	return nodeType
}

func supportedFamilies() string {
	families := make([]string, 0, len(instanceTopologies))
	for k := range instanceTopologies {
		families = append(families, k)
	}
	return strings.Join(families, ", ")
}

// ---------------------------------------------------------------------------
// Test case spec — what the user authors per test
// ---------------------------------------------------------------------------

// TestCaseClaimRef is a single entry in a test case YAML file.
type TestCaseClaimRef struct {
	Name                      string `yaml:"name"`
	ResourceClaimTemplateName string `yaml:"resourceClaimTemplateName"`
}

// TestCaseSpec is the structure of a test case YAML file.
// Each file defines the resourceClaims that a single MPIJob test should use.
// When ExpectFailure is true, the test runner treats the case as a negative test —
// it expects the MPIJob's worker pods to remain Pending (unschedulable).
type TestCaseSpec struct {
	ExpectFailure  bool               `yaml:"expectFailure"`
	ResourceClaims []TestCaseClaimRef `yaml:"resourceClaims"`
}

// ---------------------------------------------------------------------------
// ResourceClaimTemplate parsing
// ---------------------------------------------------------------------------

// ResourceClaimTemplateSpec mirrors the relevant parts of a ResourceClaimTemplate YAML.
type ResourceClaimTemplateSpec struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Spec struct {
			Devices struct {
				Requests []struct {
					Name            string `yaml:"name"`
					DeviceClassName string `yaml:"deviceClassName"`
					AllocationMode  string `yaml:"allocationMode"`
					Count           int    `yaml:"count"`
				} `yaml:"requests"`
			} `yaml:"devices"`
		} `yaml:"spec"`
	} `yaml:"spec"`
}

// ---------------------------------------------------------------------------
// MPIJob rendering
// ---------------------------------------------------------------------------

// ResourceClaimRef holds the name and template name for a single resource claim
// in the rendered MPIJob.
type ResourceClaimRef struct {
	Name         string
	TemplateName string
}

// MPIJobParams holds all template parameters for rendering the MPIJob YAML.
type MPIJobParams struct {
	SlotsPerWorker  int
	TotalRanks      int
	WorkerReplicas  int
	ContainerTestImage string
	ResourceClaims  []ResourceClaimRef
}


// RenderMPIJobYAML renders the embedded MPIJob Go template with the given params
// and returns the resulting YAML bytes.
func RenderMPIJobYAML(params MPIJobParams) ([]byte, error) {
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
// Parsing helpers
// ---------------------------------------------------------------------------

// ParseTestCaseSpec parses YAML bytes into a TestCaseSpec.
// It returns an error if the YAML is invalid or contains no resourceClaims.
func ParseTestCaseSpec(data []byte) (*TestCaseSpec, error) {
	var tc TestCaseSpec
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, fmt.Errorf("parsing test case YAML: %w", err)
	}
	if len(tc.ResourceClaims) == 0 {
		return nil, fmt.Errorf("test case has no resourceClaims")
	}
	return &tc, nil
}

func isYAMLFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

// loadRCTIndex scans a directory of RCT YAML files from the given fs.FS and
// returns a map of metadata.name → parsed spec.
func loadRCTIndex(fsys fs.FS, dir string) (map[string]*ResourceClaimTemplateSpec, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("reading RCT directory %s: %w", dir, err)
	}
	index := make(map[string]*ResourceClaimTemplateSpec)
	for _, entry := range entries {
		if entry.IsDir() || !isYAMLFile(entry.Name()) {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var rct ResourceClaimTemplateSpec
		if err := yaml.Unmarshal(data, &rct); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		index[rct.Metadata.Name] = &rct
	}
	return index, nil
}

// getNeuronCount returns the neuron device count from an RCT.
// For AllocationMode "All" it returns the topology's AllNeuronCount;
// otherwise it returns the explicit Count from the neuron request.
func getNeuronCount(rct *ResourceClaimTemplateSpec, topo *InstanceTopology) int {
	for _, req := range rct.Spec.Spec.Devices.Requests {
		if req.DeviceClassName != "neuron.aws.com" {
			continue
		}
		if req.AllocationMode == "All" {
			return topo.AllNeuronCount
		}
		return req.Count
	}
	return 0
}

// ComputeMPIJobParamsFromTestCase computes MPIJob parameters from a test case spec.
// It resolves each claim's resourceClaimTemplateName against the RCT index to
// get the neuron count, then calculates SlotsPerWorker and TotalRanks.
func ComputeMPIJobParamsFromTestCase(tc *TestCaseSpec, rctIndex map[string]*ResourceClaimTemplateSpec, topo *InstanceTopology, workerReplicas int, containerTestImage string) (*MPIJobParams, error) {
	if topo == nil {
		return nil, fmt.Errorf("instance topology is required")
	}
	if workerReplicas <= 0 {
		return nil, fmt.Errorf("workerReplicas must be positive, got %d", workerReplicas)
	}
	if containerTestImage == "" {
		return nil, fmt.Errorf("containerTestImage is required")
	}

	totalNeurons := 0
	var claims []ResourceClaimRef

	for _, tcClaim := range tc.ResourceClaims {
		rct, ok := rctIndex[tcClaim.ResourceClaimTemplateName]
		if !ok {
			return nil, fmt.Errorf("resource claim template %q not found in RCT index", tcClaim.ResourceClaimTemplateName)
		}

		totalNeurons += getNeuronCount(rct, topo)

		claims = append(claims, ResourceClaimRef{
			Name:         tcClaim.Name,
			TemplateName: tcClaim.ResourceClaimTemplateName,
		})
	}

	slotsPerWorker := totalNeurons * topo.NeuronCoresPerDevice
	totalRanks := slotsPerWorker * workerReplicas

	return &MPIJobParams{
		SlotsPerWorker:  slotsPerWorker,
		TotalRanks:      totalRanks,
		WorkerReplicas:  workerReplicas,
		ContainerTestImage: containerTestImage,
		ResourceClaims:  claims,
	}, nil
}
