

<hr>


## [v1.5.3](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.3) (2020-10-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.5.2...v1.5.3).

### `eks`

- Fix [`eks/cluster` status update panic issue](https://github.com/aws/aws-k8s-tester/pull/172).
- Expanded [VPC default CIDR range in order to support more pods for larger scale tests](https://github.com/aws/aws-k8s-tester/pull/175).
  - Previous VPC defaults had one /19 CIDR Block, allowing for 8k pods. Added multiple blocks of max VPC Block size (/16).
  - Changed VPCs from 192 space to 10 space.

### `eks/clusterloader2`

- Allowed to [specify which type of node to place the `clusterloader2` pod](https://github.com/aws/aws-k8s-tester/pull/175) (as to not have the pod be removed in scale down).

### `eksconfig`

- Use [EKS 1.18]() by default.
- Subnets are by default [same CIDR range as VPC Blocks](https://github.com/aws/aws-k8s-tester/pull/175), but can be changed with environment variables.
  - Public Subnets are /16 blocks by default.
  - Private Subnets are /17 blocks by default.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.34.22`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.22) to [`v1.35.7`](https://github.com/aws/aws-sdk-go/releases/tag/v1.35.7).

### Go

- Compile with [*Go 1.15.2*](https://golang.org/doc/devel/release.html#go1.15).


<hr>


## [v1.5.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.2) (2020-09-12)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.5.1...v1.5.2).

### `eks`

- Fix [`eks/mng` `desiredSize` parameter in CloudFormation](https://github.com/aws/aws-k8s-tester/pull/170).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.34.20`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.20) to [`v1.34.22`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.22).

### Go

- Compile with [*Go 1.15.2*](https://golang.org/doc/devel/release.html#go1.15).



<hr>




## [v1.5.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.1) (2020-09-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.5.0...v1.5.1).

### `eks`

- Fix [`eks/mng` `desiredSize` parameter in CloudFormation](https://github.com/aws/aws-k8s-tester/pull/168).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.34.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.18) to [`v1.34.20`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.20).

### Go

- Compile with [*Go 1.15.2*](https://golang.org/doc/devel/release.html#go1.15).



<hr>



## [v1.5.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.0) (2020-09-04)

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
- Fix [`eks/ng` to include `--dns-cluster-ip` in bootstrap scripts](https://github.com/aws/aws-k8s-tester/pull/162).
  - See https://github.com/awslabs/amazon-eks-ami/releases/tag/v20200821.

### `Makefile`

- Improve [build targets](https://github.com/aws/aws-k8s-tester/pull/135).

### `hack`

- Rename [`scripts` to `hack` for parity with Kubernetes projects](https://github.com/aws/aws-k8s-tester/pull/136).

### `pkg/aws`

- Add [`pkg/aws/ec2.WaitUntilRunning`](https://github.com/aws/aws-k8s-tester/pull/153).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.8) to [`v1.34.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.34.18).
- Upgrade [`github.com/kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/releases) from [`v1.18.7-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.7-rc.0) to [`v1.18.9-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.9-rc.0).
- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.7-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.18.7-rc.0) to [`v0.18.9-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.18.9-rc.0).
- Upgrade [`github.com/uber-go/zap`](https://github.com/uber-go/zap/releases) from [`v1.15.0`](https://github.com/uber-go/zap/releases/tag/v1.15.0) to [`v1.16.0`](https://github.com/uber-go/zap/releases/tag/v1.16.0).

### Go

- Compile with [*Go 1.15.1*](https://golang.org/doc/devel/release.html#go1.15).



