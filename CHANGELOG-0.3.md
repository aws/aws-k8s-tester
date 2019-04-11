

<hr>


## [0.3.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.0)(2019-04-TBD)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.2.9...0.3.0).

### `ec2config`

- Add [`InstanceProfileFilePath` field](https://github.com/aws/aws-k8s-tester/commit/78ef8e10a6a4a09456a4895f0b30a3b8f5ca8d2b).
- Add [`VolumeSize` field](https://github.com/aws/aws-k8s-tester/commit/c2d4e39af832e9369c801cfcd5fd97dbf1e41d43).
- Add [`Tags` field](https://github.com/aws/aws-k8s-tester/commit/c8b6f67a7bb712b89a4d08c4afcd00c240ba4051).

### `eksconfig`

- Add [`SSHCommands` method](https://github.com/aws/aws-k8s-tester/commit/f2ba0a997054282045deb042c38fbb3d63212eb9).
- Add [`WorkerNodeASGDesiredCapacity` field to configure `NodeAutoScalingGroupDesiredCapacity` for EKS worker nodes](https://github.com/aws/aws-k8s-tester/commit/dd2764bf29b242b4313ee1b4a16b3c592b84c6bb).
- Remove [`TestMode` field](https://github.com/aws/aws-k8s-tester/commit/c55ffe8c79f866774e1f684007b9d610769cea6d).
- Rename [`EKSCustomEndpoint` field to `EKSCustomEndpoint`](https://github.com/aws/aws-k8s-tester/commit/a3a700700b8708be6f34a1896b3b8793e602db6d).
- Move [`CFStackVPC*` to `eksconfig`, and add custom VPC/Subnet CIDR ranges](https://github.com/aws/aws-k8s-tester/commit/1a59b454beb24a6f088d1e374ccb25768341d2e9).
- Make [EKS 1.12 by default](https://github.com/aws/aws-k8s-tester/commit/bd4f3a2bfac3d635933ee614a0423efdb5b504f3).

### `internal`

- Clean up [`internal/eks`](https://github.com/aws/aws-k8s-tester/commit/a3c5696236d507160c575f134ac3958462996b9b).
- Refactor [`internal/csi` test package](https://github.com/aws/aws-k8s-tester/commit/ac63cc9b3a5ae806b8b5bd8b8d37d4a1c6208cb6).

### `pkg`

- Use [local timezone instead of UTC in `pkg/zaputil`](https://github.com/aws/aws-k8s-tester/commit/2905a5d2fdc03df9d065f876c57394d4d292b561).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.17.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.17.1) to [`v1.19.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.19.11).

### Go

- Compile with [*Go 1.12*](https://golang.org/doc/devel/release.html#go1.12).


<hr>


