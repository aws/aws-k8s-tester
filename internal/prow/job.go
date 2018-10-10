package prow

import "time"

const (
	// TypePresubmit is the spec type of Kubernetes pre-submit job.
	TypePresubmit = "pre-submit"
	// TypePostsubmit is the spec type of Kubernetes post-submit job.
	TypePostsubmit = "post-submit"
	// TypePeriodic is the spec type of Kubernetes periodic job.
	TypePeriodic = "periodic"
)

// Job represents the testing job information.
//
// There should be 1-to-1 match between GCP and AWS with combination of test category and its provider.
//
//  Columns: Type, Group, Category, Provider AWS, Provider GCP, Provider Unknown
//  Category                                  | Provider AWS                                  | Provider GCP                              | Provider Not-categorized   |
//  --------------------------------------------------------------------------------------------------------------------------------------------------------------------
//  pull-community-verify                     |                                               |                                           | pull-community-verify      |
//  pull-cri-containerd-build                 |                                               |                                           | pull-cri-containerd-build  |
//  pull-federation-bazel-test                |                                               |                                           | pull-federation-bazel-test |
//  pull-federation-e2e                       | pull-federation-e2e-aws                       | pull-federation-e2e-gce                   |                            |
//  pull-kubernetes-multicluster-ingress-test | pull-kubernetes-multicluster-ingress-test-aws | pull-kubernetes-multicluster-ingress-test |                            |
//  pull-kubernetes-e2e-kubeadm               | pull-kubernetes-e2e-kubeadm-aws               | pull-kubernetes-e2e-kubeadm-gce           |                            |
//  ci-kubernetes-kubemark-500                | ci-kubernetes-kubemark-500-aws                | ci-kubernetes-kubemark-500-gce            |                            |
//  pull-kubernetes-kubemark-e2e-big          | pull-kubernetes-kubemark-e2e-aws-big          | pull-kubernetes-kubemark-e2e-gce-big      |                            |
//  pull-kubernetes-e2e                       | pull-kubernetes-e2e-aws                       | pull-kubernetes-e2e-gce                   |                            |
//  ci-kubernetes-e2e-interconnect            | ci-kubernetes-e2e-aws-TBD-ec2                 | ci-kubernetes-e2e-gci-gce                 |                            |
//  ci-kubernetes-e2e-interconnect-k8s        | ci-kubernetes-e2e-aws-TBD-eks                 | ci-kubernetes-e2e-gci-gke                 |                            |
//  ci-kubernetes-e2e-kops                    | ci-kubernetes-e2e-kops-aws                    | ci-kubernetes-e2e-kops-gce                |                            |
//  ci-kubernetes-e2e-kops-beta               | ci-kubernetes-e2e-kops-aws-beta               | ci-kubernetes-e2e-kops-gce-beta           |                            |
//  ci-kubernetes-e2e-kops-ha-uswest2         | ci-kubernetes-e2e-kops-aws-ha-uswest2         | ci-kubernetes-e2e-kops-gce-ha-uswest2     |                            |
//  pull-cluster-api-provider-build           | pull-cluster-api-provider-aws-build           | pull-cluster-api-provider-gcp-build       |                            |
//  pull-cluster-api-provider-make            | pull-cluster-api-provider-aws-make            | pull-cluster-api-provider-gcp-make        |                            |
//  pull-cluster-api-provider-test            | pull-cluster-api-provider-aws-test            | pull-cluster-api-provider-gcp-test        |                            |
//  application-periodic-default              | application-periodic-default-eks              | application-periodic-default-gke          |                            |
//
type Job struct {
	// Type is Presubmit, Postsubmit, or Periodic.
	Type string
	// Group is the test group name.
	Group string
	// Category is the category of each test, used for platform-level aggregation.
	Category string
	// Provider is the testing infra provider.
	Provider string
	// ID is the test name, which is uniquely identifiable in test grid.
	ID string

	// Branches is the list of git branches to run tests with.
	Branches []string
	// Interval is the test run interval for type "Periodic".
	// Otherwise, set to zero.
	Interval time.Duration

	// URL is the original prow configuration path.
	URL string
	// StatusURL is the link to test results dashboard.
	StatusURL string
}
