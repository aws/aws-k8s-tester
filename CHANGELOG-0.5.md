


<hr>


## [v0.5.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.5) (2020-02-04)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.4...v0.5.5).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks create mng`](https://github.com/aws/aws-k8s-tester/commit/aac5ef7ba33ae75239473424646aa84b61a6c329).

### `eks`

- Support [multiple "Managed Node Group"s](https://github.com/aws/aws-k8s-tester/pull/74).
  - See [`eksconfig` godoc](https://godoc.org/github.com/aws/aws-k8s-tester/eksconfig) for breaking changes to environmental variable parsing.
  - See [`eksconfig` tests](https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/config_test.go) for breaking changes to environmental variable parsing.
  - See [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821) for example migration.
- Support [GPU tester](https://github.com/aws/aws-k8s-tester/commit/239fe7fa057a130c8aacc1d71bcb60d94b4ccdaf).
- Support ["Secret" tester](https://github.com/aws/aws-k8s-tester/commit/2303c8fcacae30b6b0e0bf5c4c45a2ace13f952f).
- Improve [ALB resource deletion](https://github.com/aws/aws-k8s-tester/commit/4c8f1a6fe14e1eb10ab1ac47bf98d8ee1abcf865).
  - Add [`eks/elb` package for clean-up operation](https://github.com/aws/aws-k8s-tester/commit/f4eb025f4444cf5629a8eab1e674a671ddfe0e48).
  - Retry [ELB deletion in reverse order](https://github.com/aws/aws-k8s-tester/commit/fdf60d8572c6d6720268131ba80d32c9fde2bdc9).
  - See [issue#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.
- Improve [NLB resource deletion](https://github.com/aws/aws-k8s-tester/commit/b4dc9971a6bfbf3f7356a70f7572ca4b434104cf).
  - Add [`eks/elb` package for clean-up operation](https://github.com/aws/aws-k8s-tester/commit/f4eb025f4444cf5629a8eab1e674a671ddfe0e48).
  - Retry [ELB deletion in reverse order](https://github.com/aws/aws-k8s-tester/commit/fdf60d8572c6d6720268131ba80d32c9fde2bdc9).
  - See [issue#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.
- Update [ALB Ingress Controller default image from `v1.1.3` to `v1.1.5`](https://github.com/aws/aws-k8s-tester/commit/041907e35ba88daf708b6282a77f4c1a5ada8782).
- Fix [NLB cluster role policy for `eksconfig.AddOnNLBHelloWorld`](https://github.com/aws/aws-k8s-tester/commit/aa8d56335169395ba23362119bddac7bdd447273).

### `eksconfig`

- Support [multiple "Managed Node Group"s](https://github.com/aws/aws-k8s-tester/pull/74).
  - See [`eksconfig` godoc](https://godoc.org/github.com/aws/aws-k8s-tester/eksconfig) for breaking changes to environmental variable parsing.
  - See [`eksconfig` tests](https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/config_test.go) for breaking changes to environmental variable parsing.
  - See [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821) for example migration.
- Support [GPU tester](https://github.com/aws/aws-k8s-tester/commit/239fe7fa057a130c8aacc1d71bcb60d94b4ccdaf).
- Support ["Secret" tester](https://github.com/aws/aws-k8s-tester/commit/2303c8fcacae30b6b0e0bf5c4c45a2ace13f952f).
- Use upstream [`kubectl` binary by default](https://github.com/aws/aws-k8s-tester/commit/f0a97247bf0d6d7bbc8892ab3067a2db8b7cc253).

### `pkg/awsapi/cloudformation`

- Fix [hanging `Poll` function when `DELETE_FAILED`](https://github.com/aws/aws-k8s-tester/commit/5a36d9604f09cd2a9fb1659fd7acfb4c35ef088e).
  - See [issues#69](https://github.com/aws/aws-k8s-tester/issues/69) for more details.

### `etcd`

- Deprecate [`etcd` test packages](https://github.com/aws/aws-k8s-tester/commit/96dd6292df8768ea4243d2d9b2995b0759fe61f4).

### `csi`

- Deprecate [`csi` test packages](https://github.com/aws/aws-k8s-tester/commit/c648032f0c8405ef56563f09606b6a4d84ab5929).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.27.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.27.0) to [`v1.28.10`](https://github.com/aws/aws-sdk-go/releases/tag/v1.28.10).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.4) (2020-01-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.3...v0.5.4).

### `eks`

- Ensure [subtester jobs are torn down properly](https://github.com/aws/aws-k8s-tester/pull/72).
  - See [issues#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.3) (2020-01-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.2...v0.5.3).

### `eks`

- Fix ["unable to detach ENI that was created by aws-k8s-tester due to permissions issue"](https://github.com/aws/aws-k8s-tester/pull/71).
  - See [issues#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.26.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.26.8) to [`v1.27.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.27.0).

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.2) (2019-12-30)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.1...v0.5.2).

### `eks`

- Fix [`chmod` failures when ensuring executables](https://github.com/aws/aws-k8s-tester/pull/67).
  - Fix [aws-k8s-tester#66](https://github.com/aws/aws-k8s-tester/issues/66).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.25.47`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.47) to [`v1.26.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.26.8).

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.1) (2019-12-08)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.0...v0.5.1).

### `aws-k8s-tester`

- Add [`arm64` builds to release](https://github.com/aws/aws-k8s-tester/commit/39a6fcc687a45590b594e285708d9d03287873e5).

### `eks`

- Fix [delete operation](https://github.com/aws/aws-k8s-tester/commit/08efbaedf32ed84979623e4129acafbee6eaea5f).

### `eksconfig`

- Add [`Parameters.ManagedNodeGroupCreate`](https://github.com/aws/aws-k8s-tester/commit/9498e7093ba0696d96a87dca843ff68c6561bb02).

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


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


