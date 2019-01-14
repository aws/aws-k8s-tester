

<hr>



## [0.2.2](https://github.com/aws/aws-k8s-tester/releases/tag/0.2.2)(2018-01-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.2.1...0.2.2).

### `eksconfig`

- Add [more regions](https://github.com/aws/aws-k8s-tester/commit/630b0a02f7f9dad07ac5492f34f5be006c45138e).

### Go

- Compile with [*Go 1.11.4*](https://golang.org/doc/devel/release.html#go1.11).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.16.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.15) to [`v1.16.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.18).


<hr>


## [0.2.1](https://github.com/aws/aws-k8s-tester/releases/tag/0.2.1)(2018-01-08)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.2.0...0.2.1).

### `eksconfig`

- Add [`eksconfig.EnableWorkerNodePrivilegedPortAccess`](https://github.com/aws/aws-k8s-tester/pull/33).

### Go

- Compile with [*Go 1.11.4*](https://golang.org/doc/devel/release.html#go1.11).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.16.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.14) to [`v1.16.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.15).


<hr>


## [0.2.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.2.0)(2018-01-07)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.9...0.2.0).

### `aws-k8s-tester`

- Add [`aws-k8s-tester csi test integration --github-account` flag](https://github.com/aws/aws-k8s-tester/pull/31).
- Add [`aws-k8s-tester csi test integration --github-branch` flag](https://github.com/aws/aws-k8s-tester/pull/31).
- Add [`aws-k8s-tester csi test integration --pr-num` flag](https://github.com/aws/aws-k8s-tester/pull/31).
- Remove [`aws-k8s-tester csi test integration --csi` flag](https://github.com/aws/aws-k8s-tester/pull/31).

### `kubernetesconfig`

Experimenting.

- [Initial commits to run Kubernetes e2e tests with vanilla Kubernetes cluster on top of AWS](https://github.com/aws/aws-k8s-tester/pull/26).

### `ec2config`

- Add [`ec2config.CustomScript` field to support custom CSI EBS volume install scripts](https://github.com/aws/aws-k8s-tester/pull/31).
- Use [`amazon-linux-extras install` command to install Docker in Amazon Linux 2](https://github.com/aws/aws-k8s-tester/commit/f9d9aa93e989f74ddce5ec87f126b55447c2bf9a).
- Shorten [AWS resource tag prefix from `"awsk8stester-"` to `"a8t-"`](https://github.com/aws/aws-k8s-tester/commit/5cd0e6c0d7ec73e4d647db2c5b70f0e019994c06).

### `etcdconfig`

- Use [`"AWS_K8S_TESTER_EC2_ETCD_NODES_"` and `"AWS_K8S_TESTER_EC2_ETCD_BASTION_NODES_"` for etcd environmental variable configuration prefix](https://github.com/aws/aws-k8s-tester/commit/fd9545d6acd56a2c1c0eef4da344014af7eb266a).
- Shorten [AWS resource tag prefix from `"awsk8stester-"` to `"a8t-"`](https://github.com/aws/aws-k8s-tester/commit/5cd0e6c0d7ec73e4d647db2c5b70f0e019994c06).
- Add [`"etcd"` tag to every etcd flag field](https://github.com/aws/aws-k8s-tester/commit/caac7dee6e5984ba92c340addd0404edeb4bf0cd).

### `eksconfig`

- Add [`eksconfig.UploadKubeConfig` field to disable `KUBECONFIG` S3 bucket upload by default](https://github.com/aws/aws-k8s-tester/commit/73f6c8037c949cfca03be4e776c06f9c1c76b6a0).
- Shorten [AWS resource tag prefix from `"awsk8stester-"` to `"a8t-"`](https://github.com/aws/aws-k8s-tester/commit/5cd0e6c0d7ec73e4d647db2c5b70f0e019994c06).

### `internal`

- Support [custom script for `internal/ec2` cloud init operation](https://github.com/aws/aws-k8s-tester/pull/31).
- Remove [`internal/kubeadm`](https://github.com/aws/aws-k8s-tester/commit/aa0590623f0b537484720d49175044661eda7cdb).
- Add [`internal/kubernetes` to run Kubernetes e2e tests with vanilla Kubernetes cluster on top of AWS](https://github.com/aws/aws-k8s-tester/pull/26).
- Remove [`internal/eks` `"aws-cli"` option for now](https://github.com/aws/aws-k8s-tester/commit/8079d8a96c85f2edc57da87c8b839ba67fd67f64).
- Simplify [`internal/eks` roll-back operation in `"Up"` call](https://github.com/aws/aws-k8s-tester/commit/91f9f9bc1dc88520e68a73fb132e37bfac34e6ba).
- Remove [hard-coded `kubectl` and `aws-iam-authenticator` paths in `internal/eks`](https://github.com/aws/aws-k8s-tester/commit/b8a5508589c08b9b1f256991d0d8e7513bdea5b8).
- Allow [`internal/ec2` to reuse existing SSH keys](https://github.com/aws/aws-k8s-tester/commit/99459f742ff78ba061b4cf9ef17fa697ee070613).
- Make [`internal/ec2` logging less verbose](https://github.com/aws/aws-k8s-tester/commit/1ad8b1c1718874ea51812583d5463863db4617a9).
- Make [`kubectl cluster-info dump` output less verbose](https://github.com/aws/aws-k8s-tester/commit/9a7775552ecad300783e609a0ed3677e87f2e54e).
- Make [`internal/ssh` `"verbose"` field `false` by default](https://github.com/aws/aws-k8s-tester/commit/1ad8b1c1718874ea51812583d5463863db4617a9).
- Return [error on `internal/etcd` `"MemberAdd"` operation failure](https://github.com/aws/aws-k8s-tester/commit/d03985668fd0afbabb43f46269c6daf2a779d376).

### Other

- Update default [Amazon Linux 2 AMI from `amzn2-ami-hvm-2.0.20181024-x86_64-gp2` to `amzn2-ami-hvm-2.0.20181114-x86_64-gp2`](https://github.com/aws/aws-k8s-tester/commit/b66c4b82a10ea48ff8889eb07b3530ce1fb98d5d).
  - From `Amazon Linux 2 AMI (HVM), SSD Volume Type, amzn2-ami-hvm-2.0.20181024-x86_64-gp2` to `Amazon Linux 2 AMI (HVM), SSD Volume Type, amzn2-ami-hvm-2.0.20181114-x86_64-gp2`.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.16.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.11) to [`v1.16.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.16.14).

### Go

- Compile with [*Go 1.11.4*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

