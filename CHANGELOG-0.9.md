

<hr>


## [v0.9.8](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.8) (2020-04-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.7...v0.9.8).

### `eksconfig`

- Fix [`eks create config` by removing unnecessary `eksconfig.Config.Sync` call](https://github.com/aws/aws-k8s-tester/pull/83).
  - https://github.com/aws/aws-k8s-tester/issues/82

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.2) to [`v1.30.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.3).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.7](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.7) (2020-04-01)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.6...v0.9.7).

### `eksconfig`

- Fix [`Sync` method to not overwriting pointer fields with `nil`](https://github.com/aws/aws-k8s-tester/commit/2a2aa2a9428161624c6a20126a940b40d31dbae4).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.1) to [`v1.30.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.2).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.6](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.6) (2020-03-31)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.5...v0.9.6).

### `eksconfig`

- Clean up [`KubectlCommands` output](https://github.com/aws/aws-k8s-tester/commit/76b35f487480290d344f918ddd5b0cb99566831d).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.0) to [`v1.30.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.1).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.5) (2020-03-30)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.4...v0.9.5).

### `eks`

- Improve [`NG` and `MNG` polling, check in reverse order of creation time for each ASG](https://github.com/aws/aws-k8s-tester/commit/ac69bebef621271526d07edf0f23a3e96d32f459).
- Improve [`Poll` functions in case stack has already been created complete](https://github.com/aws/aws-k8s-tester/commit/ac69bebef621271526d07edf0f23a3e96d32f459).

### `pkg/aws`

- Improve [`cloudformation.Poll` functions in case stack has already been created complete](https://github.com/aws/aws-k8s-tester/commit/ac69bebef621271526d07edf0f23a3e96d32f459).

### `eksconfig`

- Simplify [`KubectlCommands` output](https://github.com/aws/aws-k8s-tester/commit/d890ee138d1f63f2a8c2697163c9dc2fb2a69361).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.34`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.34) to [`v1.30.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.0).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.4) (2020-03-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.3...v0.9.4).

### `eks`

- List [`csr` while waiting for ASG](https://github.com/aws/aws-k8s-tester/commit/41202c1501602a88894b7e6cf3ec1235fda320b6).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.32`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.32) to [`v1.29.34`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.34).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.3) (2020-03-25)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.2...v0.9.3).

### `eks`

- Set [default `kubectl` download path to `1.16`](https://github.com/aws/aws-k8s-tester/commit/0f21c40dd8ecc3d552d64eba1ac3b6eaf368694b).
- Skip [`kubectl` and `aws-iam-authenticator` download if exists](https://github.com/aws/aws-k8s-tester/commit/0f21c40dd8ecc3d552d64eba1ac3b6eaf368694b).

### `ekstester`

- [Deprecate `ekstester` package](https://github.com/aws/aws-k8s-tester/commit/a6cc130e951d78075c7963222b805d4c55312e1c).
  - See [`test-infra#16890`](https://github.com/kubernetes/test-infra/pull/16890).
  - Upstream `k8s.io/test-infra` has deprecated old `ekstester`.
  - Fix https://github.com/aws/aws-k8s-tester/issues/73.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.30`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.30) to [`v1.29.32`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.32).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.2) (2020-03-24)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.1...v0.9.2).

### `eks`

- Improve [`eks/alb` `ELB2API.DescribeLoadBalancers` error logging](https://github.com/aws/aws-k8s-tester/commit/6af497890100d8980e801d18ca1aab5b943aa86d).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.29`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.29) to [`v1.29.30`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.30).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.1) (2020-03-23)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.0...v0.9.1).

### `ec2`

- Set [`MapPublicIpOnLaunch` to `true` in public subnet creation](https://github.com/aws/aws-k8s-tester/commit/68ed5183f41635d3c9cf9570970cece0da450251).
  - Public Subnets must have egress.
  - Otherwise, it will error as:
  - > "code": "Ec2SubnetInvalidConfiguration", "message": "One or more Amazon EC2 Subnets of [subnet-0cd62ce8f19b1817c, subnet-0b0dce45d938751fd, subnet-0310bcff99bfdf415] for node group leegyuho-test-eks-mng-al2-cpu does not automatically assign public IP addresses to instances launched into it. If you want your instances to be assigned a public IP address, then you need to enable auto-assign public IP address for the subnet. See IP addressing in VPC guide: https://docs.aws.amazon.com/vpc/latest/userguide/vpc-ip-addressing.html#subnet-public-ip",

### `eks`

- Set [`MapPublicIpOnLaunch` to `true` in public subnet creation](https://github.com/aws/aws-k8s-tester/commit/68ed5183f41635d3c9cf9570970cece0da450251).
  - Public Subnets must have egress.
  - Otherwise, it will error as:
  - > "code": "Ec2SubnetInvalidConfiguration", "message": "One or more Amazon EC2 Subnets of [subnet-0cd62ce8f19b1817c, subnet-0b0dce45d938751fd, subnet-0310bcff99bfdf415] for node group leegyuho-test-eks-mng-al2-cpu does not automatically assign public IP addresses to instances launched into it. If you want your instances to be assigned a public IP address, then you need to enable auto-assign public IP address for the subnet. See IP addressing in VPC guide: https://docs.aws.amazon.com/vpc/latest/userguide/vpc-ip-addressing.html#subnet-public-ip",

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.9.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.0) (2020-03-22)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.8...v0.9.0).

### `ec2config`

- Include [more fields from `*ec2.Instance` when `ConvertInstance`](https://github.com/aws/aws-k8s-tester/commit/4da03155db15ed1880da46a51c69db1ad04959dd).

### `eksconfig`

- Add [`MNG.ASGName` field](https://github.com/aws/aws-k8s-tester/commit/bc9f04c99baf1a4914bed4689fae308f536f247f).
- Skip [`SSHCommands` if no instance is found](https://github.com/aws/aws-k8s-tester/commit/1f0006cbe07079d4ebeda78697c8cd4750dd6a63).

### `eks`

- Improve [node group waits using EC2 Private DNS](https://github.com/aws/aws-k8s-tester/commit/eafc1f84c2096b07edcb501d1ddfa99f6c545d64).
- Run [`kubectl get nodes` while waiting for node groups](https://github.com/aws/aws-k8s-tester/commit/ed19ebf6b7abde641552273e35bd2f7a8a1d86fd).
- Return [an error if `MNG` creation fails with `CREATE_FAILED`](https://github.com/aws/aws-k8s-tester/commit/74ca7e997050971795b8f2d3b5513db00688c988).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

