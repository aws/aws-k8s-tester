package common

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

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
// MPIJob rendering helpers
// ---------------------------------------------------------------------------

// ResourceClaimRef holds the name and template name for a single resource claim
// in the rendered MPIJob.
type ResourceClaimRef struct {
	Name         string
	TemplateName string
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

// IsYAMLFile reports whether the given filename has a .yaml or .yml extension.
func IsYAMLFile(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

// LoadRCTIndex scans a directory of RCT YAML files from the given fs.FS and
// returns a map of metadata.name → parsed spec.
func LoadRCTIndex(fsys fs.FS, dir string) (map[string]*ResourceClaimTemplateSpec, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("reading RCT directory %s: %w", dir, err)
	}
	index := make(map[string]*ResourceClaimTemplateSpec)
	for _, entry := range entries {
		if entry.IsDir() || !IsYAMLFile(entry.Name()) {
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

// ExtractFamily extracts the instance family prefix from a node type string
// (before the first "."). For example, "trn1.32xlarge" returns "trn1".
func ExtractFamily(nodeType string) string {
	if idx := strings.Index(nodeType, "."); idx > 0 {
		return nodeType[:idx]
	}
	return nodeType
}

// ---------------------------------------------------------------------------
// Runtime helpers
// ---------------------------------------------------------------------------

// SplitImageRepoTag splits a container image reference on the last ":" into
// repository and tag. If there is no ":", the entire string is treated as the
// repository and the tag defaults to "latest".
func SplitImageRepoTag(image string) (repo, tag string) {
	idx := strings.LastIndex(image, ":")
	if idx < 0 {
		return image, "latest"
	}
	return image[:idx], image[idx+1:]
}

// ValidateRequiredFlags validates that all flag values in the provided map are
// non-empty. Returns a descriptive error for the first missing flag, or nil if
// all flags are present.
func ValidateRequiredFlags(flags map[string]string) error {
	for name, value := range flags {
		if value == "" {
			return fmt.Errorf("-%s is required and must be non-empty", name)
		}
	}
	return nil
}

// LoadRCTManifests reads all YAML files from the given RCT subdirectory in an
// embedded filesystem and returns them as raw byte slices suitable for
// fwext.ApplyManifests.
func LoadRCTManifests(fsys fs.FS, rctSubDir string) ([][]byte, error) {
	entries, err := fs.ReadDir(fsys, rctSubDir)
	if err != nil {
		return nil, fmt.Errorf("reading RCT directory %s: %w", rctSubDir, err)
	}
	var manifests [][]byte
	for _, entry := range entries {
		if entry.IsDir() || !IsYAMLFile(entry.Name()) {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(rctSubDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		manifests = append(manifests, data)
	}
	return manifests, nil
}
