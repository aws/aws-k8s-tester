

<hr>

## [v0.5.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.3) (2020-01-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.2...v0.5.3).

### `eks`

- Fix ["unable to detach ENI that was created by aws-k8s-tester due to permissions issue"](https://github.com/aws/aws-k8s-tester/pull/71).
  - See [issues#70](https://github.com/aws/aws-k8s-tester/issues/70) for more detail.

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


