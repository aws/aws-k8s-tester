

<hr>


## [0.3.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.3.0)(2019-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.2.8...0.3.0).

### `ec2config`

- Add [`Tags` field](https://github.com/aws/aws-k8s-tester/commit/c8b6f67a7bb712b89a4d08c4afcd00c240ba4051).

### `eksconfig`

- Add [`WorkerNodeASGDesiredCapacity` field to configure `NodeAutoScalingGroupDesiredCapacity` for EKS worker nodes](https://github.com/aws/aws-k8s-tester/commit/dd2764bf29b242b4313ee1b4a16b3c592b84c6bb).
- Remove [`TestMode` field](https://github.com/aws/aws-k8s-tester/commit/c55ffe8c79f866774e1f684007b9d610769cea6d).
- Rename [`EKSCustomEndpoint` field to `EKSCustomEndpoint`](https://github.com/aws/aws-k8s-tester/commit/a3a700700b8708be6f34a1896b3b8793e602db6d).

### `internal`

- Clean up [`internal/eks`](https://github.com/aws/aws-k8s-tester/commit/a3c5696236d507160c575f134ac3958462996b9b).

### `pkg`

- Use [local timezone instead of UTC in `pkg/zaputil`](https://github.com/aws/aws-k8s-tester/commit/2905a5d2fdc03df9d065f876c57394d4d292b561).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.17.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.17.1) to [`vTBD`](https://github.com/aws/aws-sdk-go/releases/tag/vTBD).

### Go

- Compile with [*Go 1.12*](https://golang.org/doc/devel/release.html#go1.12).


<hr>


