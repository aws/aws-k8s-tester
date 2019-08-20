

<hr>


## [0.3.3](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.3)(2019-08-20)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.3.2...0.3.3).

### `ec2config`

- Remove [instance type checks](https://github.com/aws/aws-k8s-tester/commit/5cd11f2c54f3956edaeeac7680fe678f9340e73f).
  - The invalid instance type will return errors from AWS API anyway.

### `eksconfig`

- Remove [instance type, region, Kubernetes version checks](https://github.com/aws/aws-k8s-tester/commit/5cd11f2c54f3956edaeeac7680fe678f9340e73f).
  - The invalid input will return errors from AWS API anyway.

### `eks`

- Clean up [EKS auth code](https://github.com/aws/aws-k8s-tester/blob/a686ab5d6ec72f016b3b6dab843a532397fdc78a/eks/eks_auth.go).


<hr>


## [0.3.2](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.2)(2019-08-15)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.3.1...0.3.2).

### `aws-k8s-tester alb-log`

- Remove, to be added back in future releases.

### `aws-k8s-tester wrk`

- Remove, to be added back in future releases.

### `ec2config`

- Replace [`LogDebug` with `LogLevel`](https://github.com/aws/aws-k8s-tester/commit/83140d408676142f7e5e7a2fe9cd0c19e8aec6bf).

### `eksconfig`

- Replace [`LogDebug` with `LogLevel`](https://github.com/aws/aws-k8s-tester/commit/83140d408676142f7e5e7a2fe9cd0c19e8aec6bf).

### `ekstester`

- Add [`KubernetesClientSet() *kubernetes.Clientset`](https://github.com/aws/aws-k8s-tester/commit/b5eaf2c0ec3215366d4211e68c0a3c118cd29e8b) method to `ekstester.Deployer` interface.
  - Easily extendable for other projects.
  - See https://github.com/aws/aws-k8s-tester/issues/48 for more.

### `etcdconfig`

- Replace [`LogDebug` with `LogLevel`](https://github.com/aws/aws-k8s-tester/commit/83140d408676142f7e5e7a2fe9cd0c19e8aec6bf).

### `kubernetesconfig`

- Remove, to be added back in future releases.

### `kubeadmconfig`

- Remove, to be added back in future releases.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.22.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.22.0) to [`v1.23.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.23.1).

### Go

- Compile with [*Go 1.12*](https://golang.org/doc/devel/release.html#go1.12).


<hr>


## [0.3.1](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.1)(2019-08-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.3.0...0.3.1).

### `internal`

- Use [Go 1.12.7 for CSI integration tests](https://github.com/aws/aws-k8s-tester/commit/3b052ededa5a0cc37ac145fab31556bb463b9a3a).

### Go

- Compile with [*Go 1.12*](https://golang.org/doc/devel/release.html#go1.12).


<hr>


## [0.3.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.0)(2019-08-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.2.8...0.3.0).

### `ec2config`

- Add [`InstanceProfileFilePath` field](https://github.com/aws/aws-k8s-tester/commit/78ef8e10a6a4a09456a4895f0b30a3b8f5ca8d2b).
- Add [`VolumeSize` field](https://github.com/aws/aws-k8s-tester/commit/c2d4e39af832e9369c801cfcd5fd97dbf1e41d43).
- Add [`Tags` field](https://github.com/aws/aws-k8s-tester/commit/c8b6f67a7bb712b89a4d08c4afcd00c240ba4051).
- Remove [`BackupConfig` method](https://github.com/aws/aws-k8s-tester/commit/48e009b185b5dc10f9b5295806bf3845e5e6d4de).

### `etcdconfig`

- Remove [`BackupConfig` method](https://github.com/aws/aws-k8s-tester/commit/48e009b185b5dc10f9b5295806bf3845e5e6d4de).

### `eks`

- Move [out of `internal` package](https://github.com/aws/aws-k8s-tester/commit/b4015a63d24887f06c7ec9e42c1ea5ac5e8d1831).
  - [Use case](https://github.com/aws/aws-k8s-tester/issues/47).

### `eksconfig`

- Add [`SSHCommands` method](https://github.com/aws/aws-k8s-tester/commit/f2ba0a997054282045deb042c38fbb3d63212eb9).
- Add [`KubectlCommands` method](https://github.com/aws/aws-k8s-tester/commit/00eda4d5a5edba78e08d607d2891aea632ac0e46).
- Add [`WorkerNodeASGDesiredCapacity` field to configure `NodeAutoScalingGroupDesiredCapacity` for EKS worker nodes](https://github.com/aws/aws-k8s-tester/commit/dd2764bf29b242b4313ee1b4a16b3c592b84c6bb).
- Remove [`TestMode` field](https://github.com/aws/aws-k8s-tester/commit/c55ffe8c79f866774e1f684007b9d610769cea6d).
- Rename [`EKSCustomEndpoint` field to `EKSCustomEndpoint`](https://github.com/aws/aws-k8s-tester/commit/a3a700700b8708be6f34a1896b3b8793e602db6d).
- Move [`CFStackVPC*` to `eksconfig`, and add custom VPC/Subnet CIDR ranges](https://github.com/aws/aws-k8s-tester/commit/6df3c2497127da9bf06794c5519e4e4b245764af).
- Make [EKS 1.13 by default](https://github.com/aws/aws-k8s-tester/commit/933d7ac1475b991e02aad2b2681c2a60cf7a2e16).
- Upgrade to [CNI 1.5](https://github.com/aws/aws-k8s-tester/commit/933d7ac1475b991e02aad2b2681c2a60cf7a2e16).
- Remove all [ALB plugin code](https://github.com/aws/aws-k8s-tester/commit/229c321b8a9a044a1726d4c23e7383036e36b753).
- Remove [`BackupConfig` method](https://github.com/aws/aws-k8s-tester/commit/48e009b185b5dc10f9b5295806bf3845e5e6d4de).

### `internal`

- Clean up [`internal/eks`](https://github.com/aws/aws-k8s-tester/commit/a3c5696236d507160c575f134ac3958462996b9b).
- Refactor [`internal/csi` test package](https://github.com/aws/aws-k8s-tester/commit/ac63cc9b3a5ae806b8b5bd8b8d37d4a1c6208cb6).
- Remove all [ALB plugin code](https://github.com/aws/aws-k8s-tester/commit/229c321b8a9a044a1726d4c23e7383036e36b753).

### `pkg`

- Use [local timezone instead of UTC in `pkg/zaputil`](https://github.com/aws/aws-k8s-tester/commit/2905a5d2fdc03df9d065f876c57394d4d292b561).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.20.20`](https://github.com/aws/aws-sdk-go/releases/tag/v1.20.20) to [`v1.22.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.22.0).
- Upgrade [`go.uber.org/zap`](https://github.com/uber-go/releases) from [`v1.10.0`](https://github.com/uber-go/zap/releases/tag/v1.10.0) to [`v1.10.0`](https://github.com/uber-go/zap/releases/tag/v1.10.0).

### Go

- Compile with [*Go 1.12*](https://golang.org/doc/devel/release.html#go1.12).


<hr>


