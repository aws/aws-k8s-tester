
`k8s-tester` implements defines Kubernetes "tester client" interface without "cluster provisioner" dependency. This replaces all test cases under `eks/*` (< `aws-k8s-tester` v1.6). The tester assumes an existing Kubernetes cluster (e.g., EKS, vanilla Kubernetes) and worker nodes to run testing components.

The test case:
 - may be opinionated but must comply with `"github.com/aws/aws-k8s-tester/k8s-tester/tester.Tester"` interface
 - may require certain AWS API calls and assume correct IAM or instance role for required AWS actions
 - must be generic enough to run against any Kubernetes cluster on AWS
 - must implement clean-up in a non-destrutive way
 - must implement a package that can be easily imported as a library (e.g., integrates with EKS tester)
 - must control their own dependencies (e.g., vending Kubernetes client-go) in case a user does not want to carry out other dependencies
 - must be easy to use -- a single command for install and clean-up
 - must implement a CLI with the sub-commands of "apply" and "delete"

To add a new tester,

**Step 1.** Create a new directory under `"k8s-tester"`.

**Step 2.** Implement `"github.com/aws/aws-k8s-tester/k8s-tester/tester.Tester"` interface.

**Step 3.** Run

```bash
go mod init
go mod tidy -v

# don't run
# go mod vendor -v
```
