

<hr>


## [v0.6.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.3) (2020-02-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.2...v0.6.3).

### `eksconfig`

- Do not support [`AddOnManagedNodeGroups` when `Parameters.Version < 1.14`](https://github.com/aws/aws-k8s-tester/commit/56e53019f65e9da585ccb98b1f1dc27de5409edf).
- Add [`AddOnFargate`](https://github.com/aws/aws-k8s-tester/commit/56e53019f65e9da585ccb98b1f1dc27de5409edf).
- Add [default log output file path to `LogOutputs`](https://github.com/aws/aws-k8s-tester/commit/56e53019f65e9da585ccb98b1f1dc27de5409edf).

### `eks`

- Fix [namespace deletion](https://github.com/aws/aws-k8s-tester/commit/f388fe3bfed6d7ef0f7d3fad237ffa3c74341df6).
  - See [issues#79](https://github.com/aws/aws-k8s-tester/issues/79).
- Fix [`MNG` delete operation](https://github.com/aws/aws-k8s-tester/commit/1f8a396b56bcf46780c62eedfd34624c2ad35d8a).
- Improve [`AddOnSecrets` pod deployment waits](https://github.com/aws/aws-k8s-tester/commit/82c62f4dc27592592b6743ad792280d13842be50).
- Add [`github.com/aws/aws-k8s-tester/eks/fargate`](https://github.com/aws/aws-k8s-tester/commit/e0b4d2820e531f04d6d9c8eabd944a6304254ea4).
- Support [private subnets in VPC creation for `eks/fargate`](https://github.com/aws/aws-k8s-tester/commit/08044d91316071381cc30ef306d2be76e8ed0260).
- Add [`github.com/aws/aws-k8s-tester/eks/metrics`](https://github.com/aws/aws-k8s-tester/commit/753c76024d519cbc27a9531a1bdbcce37ecf7f20).
- Tag [VPC with `"Network"` key](https://github.com/aws/aws-k8s-tester/commit/2f8758cb67052e4affe7d6a6a4a745819563c656).

### `ssh`

- Fix [connection error handling in dial](https://github.com/aws/aws-k8s-tester/commit/dac3ad69218e7ddd44c7d7c4993d7239a761a6cf).
- Rename package path from [`github.com/aws/aws-k8s-tester/pkg/ssh` to `github.com/aws/aws-k8s-tester/ssh`](https://github.com/aws/aws-k8s-tester/commit/dac3ad69218e7ddd44c7d7c4993d7239a761a6cf).

### `ec2`

- Fetch [latest AL2 AMI from SSM parameter](https://github.com/aws/aws-k8s-tester/commit/54b1f5e67f66eaf4f1b7bcec07d39d918dabae53).

### `kms`

- Remove [package `kms`](https://github.com/aws/aws-k8s-tester/commit/270bf13176605a57a58c20941bfa188b730909e0).

### `kmsconfig`

- Remove [package `kmsconfig`](https://github.com/aws/aws-k8s-tester/commit/270bf13176605a57a58c20941bfa188b730909e0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.4`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.4) to [`v1.29.12`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.12).
- Upgrade [`github.com/uber-go/zap`](https://github.com/uber-go/zap/releases) from [`v1.13.0`](https://github.com/uber-go/zap/releases/tag/v1.13.0) to [`v1.14.0`](https://github.com/uber-go/zap/releases/tag/v1.14.0).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.2) (2020-02-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.1...v0.6.2).

### `eksconfig`

- Set [`AddOnManagedNodeGroups.Enable` to `false` by default](https://github.com/aws/aws-k8s-tester/commit/865dcaeecd2b447a9322c38c908e359b466d0471).
- Set [`AddOnNLBHelloWorld.Enable` to `false` by default](https://github.com/aws/aws-k8s-tester/commit/c1400268aa7f2bcfccda94c8ecbc79c8f17239f7).
- Improve [`AddOnManagedNodeGroups.RoleServicePrincipals` validation](https://github.com/aws/aws-k8s-tester/commit/ac2ba073a223c683eb550c6734925eb9e10e1905).
- Add [`Parameters.VPCID` for VPC reuse](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Remove [`Parameters.PrivateSubnetIDs`](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Remove [`Parameters.ControlPlaneSecurityGroupID`](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).

### `eks`

- Support [existing VPC for cluster creation](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Set [secret write fail threshold for `AddOnSecrets`](https://github.com/aws/aws-k8s-tester/commit/03122df1d3ca71d8b00c26c7f1b4b77edce287e1).
  - 10 consecutive `Secret` write failures returns an error.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.3) to [`v1.29.4`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.4).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.6.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.1) (2020-02-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.0...v0.6.1).

### `eks`

- Improve [failure logging](https://github.com/aws/aws-k8s-tester/commit/6604cbad3d64d885b16ce3246f78e3f5bc2cbc30).

### `eksconfig`

- Update [default `KubectlPath` value](https://github.com/aws/aws-k8s-tester/commit/95e8ed790e588a8f31758d901b2f8997b04d846f).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>



## [v0.6.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.0) (2020-02-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.9...v0.6.0).

### `ec2config`

- Use [`;` for `Tags` and `IngressRulesTCP` as divider character when parsing `reflect.Map`](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### `eks`

- Test [`EKS` API availability at the beginning](https://github.com/aws/aws-k8s-tester/commit/6fd70924f4d86055ecef4f9596ecf08c4e772df3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Do [not fetch OIDC endpoints when a cluster is being deleted](https://github.com/aws/aws-k8s-tester/commit/8825e8865e934cda97c6cb65078d6b562ef17f68).
- Use [`github.com/aws/aws-sdk-go/service/eks.ClusterStatus*` for status checks](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Improve [cluster status checks with `github.com/aws/aws-sdk-go/service/eks`](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [cluster status polling](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [`mng` status checks with `github.com/aws/aws-sdk-go/service/eks`](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [`mng` status polling](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).

### `eksconfig`

- Set [initial `eksconfig.Config.Name` in `eksconfig.NewDefault` using `AWS_K8S_TESTER_EKS_NAME` (if defined)](https://github.com/aws/aws-k8s-tester/commit/11c1fa3aaa654333069d002ecf1dc1e765deca02).
  - e.g. `AWS_K8S_TESTER_EKS_NAME=${USER}-cluster aws-k8s-tester eks create config -p test.yaml`
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Disable [`AddOnALB2048` by default](https://github.com/aws/aws-k8s-tester/commit/f437b006afbc304bd1552fa143cfcd6a5cbc8e39).
- Rename [`AddOnManagedNodeGroups.LogDir` to `AddOnManagedNodeGroups.LogsDir`](https://github.com/aws/aws-k8s-tester/commit/bf3a92a97fbe4571388f7909225129fe3ee926da).
- Improve [`AddOnManagedNodeGroups.LogsDir` defaults](https://github.com/aws/aws-k8s-tester/commit/4524c52ab907152bc85c656c54864e075f7ec5f3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Fix [`VPCID` checks](https://github.com/aws/aws-k8s-tester/commit/1758b1af46b71a837653518884414619e7003550).

### `kmsconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.1) to [`v1.29.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.3).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>

