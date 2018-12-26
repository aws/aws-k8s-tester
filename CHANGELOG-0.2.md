

<hr>


## [0.2.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.2.0) (2018-12-31)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.9...0.2.0).

### `kubernetesconfig`

- [Initial commit to run Kubernetes e2e tests with vanilla Kubernetes cluster on top of AWS](https://github.com/aws/aws-k8s-tester/commit/TODO).

### `internal`

- Add [`internal/kubernetes` to run Kubernetes e2e tests with vanilla Kubernetes cluster on top of AWS](https://github.com/aws/aws-k8s-tester/commit/TODO).
- Remove [`internal/eks` `"aws-cli"` option for now](https://github.com/aws/aws-k8s-tester/commit/8079d8a96c85f2edc57da87c8b839ba67fd67f64).
- Allow [`internal/ec2` to reuse existing SSH keys](https://github.com/aws/aws-k8s-tester/commit/99459f742ff78ba061b4cf9ef17fa697ee070613).
- Make [`kubectl cluster-info dump` output less verbose](https://github.com/aws/aws-k8s-tester/commit/9a7775552ecad300783e609a0ed3677e87f2e54e).

### Other

- Update default [Amazon Linux 2 AMI from `amzn2-ami-hvm-2.0.20181024-x86_64-gp2` to `amzn2-ami-hvm-2.0.20181114-x86_64-gp2`](https://github.com/aws/aws-k8s-tester/commit/b66c4b82a10ea48ff8889eb07b3530ce1fb98d5d).
  - From `Amazon Linux 2 AMI (HVM), SSD Volume Type, amzn2-ami-hvm-2.0.20181024-x86_64-gp2` to `Amazon Linux 2 AMI (HVM), SSD Volume Type, amzn2-ami-hvm-2.0.20181114-x86_64-gp2`.

### Go

- Compile with [*Go 1.11.4*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

