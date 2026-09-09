//go:build e2e

package common

import "strings"

// GPUCountForNodeType returns the GPU count to request for pods whose
// assertions depend on node-wide GPU visibility. Callers use it as the
// value of `nvidia.com/gpu` in the pod resource request, forcing the
// device plugin to inject every GPU on the node rather than a single
// one. Instances not listed here fall through to 1, which is correct
// for single-GPU SKUs and for pods that only need one representative
// GPU (e.g. driver-liveness, CUDA sample runs).
//
// SKUs with confirmed GPU counts from AWS product pages / EC2
// describe-instance-types:
//   - p3.8xlarge: 4x V100
//   - p3.16xlarge, p3dn.24xlarge: 8x V100
//   - p4d.24xlarge, p4de.24xlarge: 8x A100
//   - p5.48xlarge, p5e.48xlarge, p5en.48xlarge: 8x H100/H200
//   - p6-b200.*, p6-b300.*: 8x B200 (Blackwell)
func GPUCountForNodeType(nodeType string) int {
	switch nodeType {
	case "p3.8xlarge":
		return 4
	case "p3.16xlarge",
		"p3dn.24xlarge",
		"p4d.24xlarge", "p4de.24xlarge",
		"p5.48xlarge", "p5e.48xlarge", "p5en.48xlarge":
		return 8
	}
	if strings.HasPrefix(nodeType, "p6-b200") || strings.HasPrefix(nodeType, "p6-b300") {
		return 8
	}
	return 1
}
