

<hr>


## [v0.5.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.0) (2019-12-05)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.4.4...v0.5.0).

### `eks`

- Rewrite to [implement `kubetest2` and use `CloudFormation`](https://github.com/aws/aws-k8s-tester/pull/64).
  - https://github.com/kubernetes/test-infra/tree/master/kubetest2
  - https://godoc.org/k8s.io/test-infra/kubetest2
  - https://godoc.org/github.com/aws/aws-k8s-tester/eksconfig
  - https://godoc.org/github.com/aws/aws-k8s-tester/eks

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.25.30`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.30) to [`v1.25.47`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.47).
- Upgrade [`go.uber.org/zap`](https://github.com/uber-go/zap/releases) from [`v1.12.0`](https://github.com/uber-go/zap/releases/tag/v1.12.0) to [`v1.13.0`](https://github.com/uber-go/zap/releases/tag/v1.13.0).
- Replace [`github.com/blang/semver/releases`](https://github.com/blang/semver/releases) with [`github.com/gyuho/semver/releases`](https://github.com/gyuho/semver/releases) [`v3.6.2`](https://github.com/gyuho/semver/releases/tag/v3.6.2).

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


