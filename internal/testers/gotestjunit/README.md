This tester executes a Go test binary and generates JUnit XML output.

It wraps the test execution, captures the output, and uses `go-junit-report` to generate a JUnit XML file in the artifacts directory.

---

This is derived from the `exec` tester and `process` package in kubetest2:

- https://github.com/kubernetes-sigs/kubetest2/tree/master/pkg/testers/exec
- https://github.com/kubernetes-sigs/kubetest2/tree/master/pkg/process

The fork originated at commit `57fcb7870313c4bd7279a41811da69128e91fa3b`.

A copy of the original license is provided in the file named `LICENSE.original`.
