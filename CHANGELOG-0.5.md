

<hr>


## [v0.5.9](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.9) (2020-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.8...v0.5.9).

### `eks`

- Improve [`Deployment` wait methods](https://github.com/aws/aws-k8s-tester/commit/7266f245e0ff52008b51a5c21a07fc65d9cfbd9e).

### `eksconfig`

- Add [`KubectlCommandsOutputPath`](https://github.com/aws/aws-k8s-tester/commit/4aa6070e762733f6b84fb6b8e6906f9dc695e068).
- Add [`SSHCommandsOutputPath`](https://github.com/aws/aws-k8s-tester/commit/4aa6070e762733f6b84fb6b8e6906f9dc695e068).

### `pkg/aws/ec2`

- Fix [a bug in batch `ec2.DescribeInstances` (used in `pkg/aws/ec2.PollUntilRunning` for `mng`)](https://github.com/aws/aws-k8s-tester/commit/5c75c7b598449c774726ac6d32ed0409237a7242).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.0) to [`v1.29.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.1).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.8](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.8) (2020-02-12)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.7...v0.5.8).

### `eks`

- Improve [`client-go` client set creation, support `kubeconfig` loader](https://github.com/aws/aws-k8s-tester/commit/67f7528abd12ed7004cc7044e3695903e22b94cf).
- Support [IAM Roles for Service Accounts (`IRSA`)](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).

### `eks/irsa`

- Initial commit to support [IAM Roles for Service Accounts (`IRSA`)](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).

### `eksconfig`

- Add [`*Config.KubectlCommand` method](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Add [`AddOnIRSA`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Rename [`eksconfig.Status.AWSARN` to `eksconfig.Status.AWSIAMRoleARN`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Rename [`eksconfig.Status.ClusterOIDCIssuer` to `eksconfig.Status.ClusterOIDCIssuerURL`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Add [`eksconfig.Status.ClusterOIDCIssuerHostPath`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Add [`eksconfig.Status.ClusterOIDCIssuerARN`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).
- Add [`eksconfig.Status.ClusterOIDCIssuerCAThumbprint`](https://github.com/aws/aws-k8s-tester/commit/b68fdbe49bd0d6e43697f75d507ba6c80e1f1dce).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.28.12`](https://github.com/aws/aws-sdk-go/releases/tag/v1.28.12) to [`v1.29.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.0).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.7](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.7) (2020-02-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.6...v0.5.7).

### `eksconfig`

- Validate [old instance types when `AddOnNLBHelloWorld` or `AddOnALB2048` is enabled](https://github.com/aws/aws-k8s-tester/commit/549ad616dc6507fd9d481a82177c0013a003926f).
  - Debug [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821).
  - See [`kubernetes/kubernetes#66044`](https://github.com/kubernetes/kubernetes/issues/66044#issuecomment-408188524).

### `ec2`

- New package `ec2` moved from [`internal/ec2`](https://github.com/aws/aws-k8s-tester/commit/afe4bf121ab941b292d1647ddb0f3448eecef71d).

### `kms`

- New package `kms` moved from [`internal/kms`](https://github.com/aws/aws-k8s-tester/commit/afe4bf121ab941b292d1647ddb0f3448eecef71d).

### `pkg/aws`

- New package `pkg/aws` moved from [`pkg/awsapi`](https://github.com/aws/aws-k8s-tester/commit/2dcae9bb901eee2905a035b263d7964ea9f6cbe0).

### `pkg/ssh`

- New package `pkg/ssh` moved from [`internal/ssh`](https://github.com/aws/aws-k8s-tester/commit/afe4bf121ab941b292d1647ddb0f3448eecef71d).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.28.10`](https://github.com/aws/aws-sdk-go/releases/tag/v1.28.10) to [`v1.28.12`](https://github.com/aws/aws-sdk-go/releases/tag/v1.28.12).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.6](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.6) (2020-02-05)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.5...v0.5.6).

### `eksconfig`

- Add [`OnFailureDelete` and `OnFailureDeleteWaitSeconds`](https://github.com/aws/aws-k8s-tester/commit/0aea513d203c5df0b86ff4448ec67a627574ae77).
- Add [`AddOnNLBHelloWorld.Namespace`](https://github.com/aws/aws-k8s-tester/commit/245ca7d53454613101c9dab188455b69c278b805).
- Add [`AddOnALB2048.Namespace`](https://github.com/aws/aws-k8s-tester/commit/245ca7d53454613101c9dab188455b69c278b805).
  - Debug [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821).

### `eks`

- Improve [`Delete` operation waits](https://github.com/aws/aws-k8s-tester/commit/4fb3060ad2695cdee3b040f332ee548222d9dcb3).
  - See [issue#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.

### `eks/nlb`

- Add [`kubectl describe svc` during host name checks](https://github.com/aws/aws-k8s-tester/commit/cb9943c6c830c2fe059330b6ce6e139ce8921e58).
  - Debug [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821).

### `eks/alb`

- Add [`kubectl describe svc` during host name checks](https://github.com/aws/aws-k8s-tester/commit/cb9943c6c830c2fe059330b6ce6e139ce8921e58).
  - Debug [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.5) (2020-02-04)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.4...v0.5.5).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks create mng`](https://github.com/aws/aws-k8s-tester/commit/aac5ef7ba33ae75239473424646aa84b61a6c329).

### `eks`

- Support [multiple "Managed Node Group"s](https://github.com/aws/aws-k8s-tester/pull/74).
  - See [`eksconfig` godoc](https://godoc.org/github.com/aws/aws-k8s-tester/eksconfig) for breaking changes to environmental variable parsing.
  - See [`eksconfig` tests](https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/config_test.go) for breaking changes to environmental variable parsing.
  - See [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821) for example migration.
- Support [GPU tester](https://github.com/aws/aws-k8s-tester/commit/239fe7fa057a130c8aacc1d71bcb60d94b4ccdaf).
- Support ["Secret" tester](https://github.com/aws/aws-k8s-tester/commit/2303c8fcacae30b6b0e0bf5c4c45a2ace13f952f).
- Improve [ALB resource deletion](https://github.com/aws/aws-k8s-tester/commit/4c8f1a6fe14e1eb10ab1ac47bf98d8ee1abcf865).
  - Add [`eks/elb` package for clean-up operation](https://github.com/aws/aws-k8s-tester/commit/f4eb025f4444cf5629a8eab1e674a671ddfe0e48).
  - Retry [ELB deletion in reverse order](https://github.com/aws/aws-k8s-tester/commit/fdf60d8572c6d6720268131ba80d32c9fde2bdc9).
  - See [issue#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.
- Improve [NLB resource deletion](https://github.com/aws/aws-k8s-tester/commit/b4dc9971a6bfbf3f7356a70f7572ca4b434104cf).
  - Add [`eks/elb` package for clean-up operation](https://github.com/aws/aws-k8s-tester/commit/f4eb025f4444cf5629a8eab1e674a671ddfe0e48).
  - Retry [ELB deletion in reverse order](https://github.com/aws/aws-k8s-tester/commit/fdf60d8572c6d6720268131ba80d32c9fde2bdc9).
  - See [issue#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.
- Update [ALB Ingress Controller default image from `v1.1.3` to `v1.1.5`](https://github.com/aws/aws-k8s-tester/commit/041907e35ba88daf708b6282a77f4c1a5ada8782).
- Fix [NLB cluster role policy for `eksconfig.AddOnNLBHelloWorld`](https://github.com/aws/aws-k8s-tester/commit/aa8d56335169395ba23362119bddac7bdd447273).

### `eksconfig`

- Support [multiple "Managed Node Group"s](https://github.com/aws/aws-k8s-tester/pull/74).
  - See [`eksconfig` godoc](https://godoc.org/github.com/aws/aws-k8s-tester/eksconfig) for breaking changes to environmental variable parsing.
  - See [`eksconfig` tests](https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/config_test.go) for breaking changes to environmental variable parsing.
  - See [`aws/amazon-vpc-cni-k8s#821`](https://github.com/aws/amazon-vpc-cni-k8s/pull/821) for example migration.
- Support [GPU tester](https://github.com/aws/aws-k8s-tester/commit/239fe7fa057a130c8aacc1d71bcb60d94b4ccdaf).
- Support ["Secret" tester](https://github.com/aws/aws-k8s-tester/commit/2303c8fcacae30b6b0e0bf5c4c45a2ace13f952f).
- Use upstream [`kubectl` binary by default](https://github.com/aws/aws-k8s-tester/commit/f0a97247bf0d6d7bbc8892ab3067a2db8b7cc253).

### `pkg/awsapi/cloudformation`

- Fix [hanging `Poll` function when `DELETE_FAILED`](https://github.com/aws/aws-k8s-tester/commit/5a36d9604f09cd2a9fb1659fd7acfb4c35ef088e).
  - See [issues#69](https://github.com/aws/aws-k8s-tester/issues/69) for more details.

### `etcd`

- Deprecate [`etcd` test packages](https://github.com/aws/aws-k8s-tester/commit/96dd6292df8768ea4243d2d9b2995b0759fe61f4).

### `csi`

- Deprecate [`csi` test packages](https://github.com/aws/aws-k8s-tester/commit/c648032f0c8405ef56563f09606b6a4d84ab5929).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.27.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.27.0) to [`v1.28.10`](https://github.com/aws/aws-sdk-go/releases/tag/v1.28.10).

### Go

- Compile with [*Go 1.13.7*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.4) (2020-01-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.3...v0.5.4).

### `eks`

- Ensure [subtester jobs are torn down properly](https://github.com/aws/aws-k8s-tester/pull/72).
  - See [issues#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.

### Go

- Compile with [*Go 1.13.5*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.5.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.5.3) (2020-01-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.2...v0.5.3).

### `eks`

- Fix ["unable to detach ENI that was created by aws-k8s-tester due to permissions issue"](https://github.com/aws/aws-k8s-tester/pull/71).
  - See [issues#70](https://github.com/aws/aws-k8s-tester/issues/70) for more details.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.26.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.26.8) to [`v1.27.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.27.0).

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


