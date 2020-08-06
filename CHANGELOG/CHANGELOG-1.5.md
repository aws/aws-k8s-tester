
<hr>



## [v1.5.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.0) (2020-07)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.8...v1.5.0).

### `ec2config`

- Set [ASG size defaults based on desired capacities](https://github.com/aws/aws-k8s-tester/pull/140).
  - Either ["desired" or "minimum" must be >0](https://github.com/aws/aws-k8s-tester/pull/143).
    - `desired 10, min 0, max  0 ==> desired 10, min 10, max 10`.
    - `desired  0, min 1, max 10 ==> desired  0, min  1, max 10`.

### `eksconfig`

- `yaml.Unmarshal` with [`yaml.DisallowUnknownFields`](https://github.com/aws/aws-k8s-tester/pull/147).
- Add [`AWS_K8S_TESTER_EKS_CONFIG`](https://github.com/aws/aws-k8s-tester/pull/138).
  - `AWS_K8S_TESTER_EKS_CONFIG` can be used in conjunction with existing `AWS_K8S_TESTER_EKS_*` environmental variables.
  - [`AWS_K8S_TESTER_EKS_CONFIG` is always loaded first](https://github.com/aws/aws-k8s-tester/pull/147) in `eksconfig`.
- Set [ASG size defaults based on desired capacities](https://github.com/aws/aws-k8s-tester/pull/140).
  - Either ["desired" or "minimum" must be >0](https://github.com/aws/aws-k8s-tester/pull/143).
    - `desired 10, min 0, max  0 ==> desired 10, min 10, max 10`.
    - `desired  0, min 1, max 10 ==> desired  0, min  1, max 10`.

### `eks`

- Add [`ClusterAutoscaler` addon with kubemark compatibility](https://github.com/aws/aws-k8s-tester/pull/137).
- Remove [unused `docker.sock`](https://github.com/aws/aws-k8s-tester/pull/141).

### `Makefile`

- Improve [build targets](https://github.com/aws/aws-k8s-tester/pull/135).

### `hack`

- Rename [`scripts` to `hack` for parity with Kubernetes projects](https://github.com/aws/aws-k8s-tester/pull/136).

### `pkg/aws`

- Add [`pkg/aws/ec2.PollASGUntilRunning`](https://github.com/aws/aws-k8s-tester/pull/153).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.8) to [`v1.33.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.18).

### Go

- Compile with [*Go 1.14.6*](https://golang.org/doc/devel/release.html#go1.14).



