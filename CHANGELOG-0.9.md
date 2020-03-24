


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

