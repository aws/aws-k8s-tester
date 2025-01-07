package util

import (
	"fmt"
	"os"
	"strings"
)

const KubernetesVersionFile = "kubernetes-version.txt"

func DetectKubernetesVersion() (string, error) {
	versionFile, err := LookPath(KubernetesVersionFile)
	if err != nil {
		return "", err
	}
	bytes, err := os.ReadFile(versionFile)
	if err != nil {
		return "", err
	}
	// "v1.2.3"
	versionTag := string(bytes)
	return strings.ReplaceAll(versionTag, "v", ""), nil
}

func ParseMinorVersion(semanticVersion string) (string, error) {
	parts := strings.Split(semanticVersion, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("malformed semantic version: '%s'", semanticVersion)
	}
	return strings.Join(parts[:2], "."), nil
}
