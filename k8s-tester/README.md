
`k8s-tester` implements defines Kubernetes "tester client" interface without "cluster provisioner" dependency. This replaces all test cases under `eks/*` (< `aws-k8s-tester` v1.6). The tester assumes an existing Kubernetes cluster (e.g., EKS, vanilla Kubernetes) and worker nodes to run testing components.

Each test case:
 - MUST comply with `"github.com/aws/aws-k8s-tester/k8s-tester/tester.Tester"` interface
 - MUST be generic enough to run against any Kubernetes cluster on AWS
 - MUST implement clean-up in a non-destrutive way
 - MUST implement a package that can be easily imported as a library (e.g., integrates with EKS tester)
 - MUST control their own dependencies (e.g., vending Kubernetes client-go) in case a user does not want to carry out other dependencies
 - MAY require certain AWS API calls and assume correct IAM or instance role for required AWS actions
 - MAY implement a CLI with the sub-commands of "apply" and "delete"

To add a new tester,
1. Create a new directory under `github.com/aws/aws-k8s-tester/k8s-tester`.
2. Implement `github.com/aws/aws-k8s-tester/k8s-tester/tester.Tester` interface within the new package `github.com/aws/aws-k8s-tester/k8s-tester/NEW-TESTER`.
3. (Optional) Implement a stand-alone CLI for the test case under `github.com/aws/aws-k8s-tester/k8s-tester/NEW-TESTER/cmd/k8s-tester-NEW-TESTER`.
4. Import the new configuration struct to `k8s-tester/config.go` with test cases in `k8s-tester/config_test.go`.
5. Add the new tester to `github.com/aws/aws-k8s-tester/k8s-tester/tester.go`.
6. Run `github.com/aws/aws-k8s-tester/k8s-tester/vend.sh`.
7. Add the new tester to `github.com/aws/aws-k8s-tester/k8s-tester/cmd/readme-gen/main.go`.
8. Run `github.com/aws/aws-k8s-tester/k8s-tester/cmd/readme-gen/vend.sh`.
9. Update and run `github.com/aws/aws-k8s-tester/k8s-tester/fmt.sh`.
10. Run `github.com/aws/aws-k8s-tester/k8s-tester/cmd/k8s-tester/vend.sh`.
11. Run `github.com/aws/aws-k8s-tester/k8s-tester/gen.sh`.

### Examples

See [`README.config.md`](./README.config.md) for all settings.

