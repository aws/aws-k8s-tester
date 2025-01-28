//go:build tools
// +build tools

package external

// this file allows us to declare direct dependencies on our required external tools.
// this file will not compile! that's expected.

import (
	_ "sigs.k8s.io/kubetest2"
	_ "sigs.k8s.io/kubetest2/kubetest2-tester-exec"
	_ "sigs.k8s.io/kubetest2/kubetest2-tester-ginkgo"
)
