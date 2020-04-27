

<hr>


## [v0.6.9](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.9) (2020-03-05)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.8...v0.6.9).

### `ec2config`

- Rewrite to use [CFN](https://github.com/aws/aws-k8s-tester/commit/33994016efbd7f514223131f5a959db50bf638ce).

### `ec2`

- Rewrite to use [CFN](https://github.com/aws/aws-k8s-tester/commit/92a6a2d5feb4ee9622b0f2d320bc754acad84790).

### `eksconfig`

- Change [field name `SSH*` to `RemoteAccess*`](https://github.com/aws/aws-k8s-tester/commit/33994016efbd7f514223131f5a959db50bf638ce).
  - Add `RemoteAccessKeyCreate` (default `true`).
  - `SSHCommandsOutputPath` is now `RemoteAccessCommandsOutputPath`.
  - `AWS_K8S_TESTER_EKS_SSH_COMMANDS_OUTPUT_PATH` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_COMMANDS_OUTPUT_PATH`.
  - `AddOnManagedNodeGroups.SSHKeyPairName` is now `AddOnManagedNodeGroups.RemoteAccessKeyName`.
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_SSH_KEY_PAIR_NAME` is now `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_KEY_NAME`.

### `eks`

- Add [Secret encryption support](https://github.com/aws/aws-k8s-tester/commit/154c6b334650cceb7f71e892f3aebfea7016ae84).
  - https://aws.amazon.com/about-aws/whats-new/2020/03/amazon-eks-adds-envelope-encryption-for-secrets-with-aws-kms/
  - https://aws.amazon.com/blogs/containers/using-eks-encryption-provider-support-for-defense-in-depth/
- Rename [`github.com/aws/aws-k8s-tester/eks/elb` to `github.com/aws/aws-k8s-tester/pkg/aws/elb`](https://github.com/aws/aws-k8s-tester/commit/87b3e79c2f5d923dd40bb9f34192ec6bf8934783).
- Add [3rd public subnet to VPC CloudFormation template](https://github.com/aws/aws-k8s-tester/commit/9de73c4eb886f94ea60e825c00ce39b6b8e61e4b).
- Check [existing ELBv2 when VPC ID is reused](https://github.com/aws/aws-k8s-tester/commit/facb0e8027ee298b8c8ce3e2ecaa05a18a70e0f7).
- Remove [existing ELBv2 when VPC is deleted](https://github.com/aws/aws-k8s-tester/commit/facb0e8027ee298b8c8ce3e2ecaa05a18a70e0f7).
- Clean up [VPC deletion](https://github.com/aws/aws-k8s-tester/commit/d3d13e226a073924b6724dee648629a4ef4ff017).
- Add [VPCName parameter to VPC template](https://github.com/aws/aws-k8s-tester/commit/2b4b461b766c4dbbc9cc2f1da3d84298c5a5a74e).

### `pkg/aws/iam`

- Fix [`AssumeRolePolicyDocument` parsing](https://github.com/aws/aws-k8s-tester/commit/624121ce66e432fcf397759b57813a6ed0cbe42e).

### `pkg/aws/elb`

- Add [`vpcID` and `tags` arguments to `DeleteELBv2`](https://github.com/aws/aws-k8s-tester/commit/b4578c016613cc07dadcb528629539fdf45a7005).
  - Support clean up with Kubernetes tags.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.14) to [`v1.29.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.18).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.8](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.8) (2020-03-01)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.7...v0.6.8).

### `eks`

- Improve [`eks/alb` Pod debugging logs](https://github.com/aws/aws-k8s-tester/commit/0ec89a60df5a01beada440e100430470d6c1a9d5).
  - Fetching ALB Ingress Controller Pod logs.
- Fix [`KUBECONFIG` path overwrite with extension](https://github.com/aws/aws-k8s-tester/commit/cebf948f76f038180f1a519d990d865eb7945d86).
- Improve [health check outputs](https://github.com/aws/aws-k8s-tester/commit/811fc2d2f219e9f63605db2e23fc82ceb7dbd9ec).
- Rename [`github.com/aws/aws-k8s-tester/eks/elb` to `github.com/aws/aws-k8s-tester/pkg/aws/elb`](https://github.com/aws/aws-k8s-tester/commit/).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.7](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.7) (2020-03-01)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.6...v0.6.7).

### `eksconfig`

- Add [`AddOnFargate.RoleCreate`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Clean up [`Config.ValidateAndSetDefaults`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Clean up [role create validation](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Remove [`Cluster` from all `Parameters.Cluster*` fields](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
  - e.g. Change `AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_` to `AWS_K8S_TESTER_EKS_PARAMETERS_`.
  - e.g. Change `AWS_K8S_TESTER_EKS_PARAMETERS_CLUSTER_SIGNING_NAME` to `AWS_K8S_TESTER_EKS_PARAMETERS_SIGNING_NAME`.
- Remove [`Status.RoleCFNStackID`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Remove [`Status.RoleARN`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Remove [`StatusManagedNodeGroup.RoleName`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Remove [`StatusManagedNodeGroup.RoleARN`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Remove [`StatusManagedNodeGroup.RoleCFNStackID`](https://github.com/aws/aws-k8s-tester/commit/5548b155d10ac8b4fc3231f0dd0f6fd77690d405).
- Add [`Config.CommandAfterCreateCluster`](https://github.com/aws/aws-k8s-tester/commit/474ecb790ae80263fbadba69fabb6ae97ea98e50).
- Add [`Config.CommandAfterCreateAddOns`](https://github.com/aws/aws-k8s-tester/commit/474ecb790ae80263fbadba69fabb6ae97ea98e50).

### `eks`

- Fix [health check output](https://github.com/aws/aws-k8s-tester/commit/05f0101effb4b776b4f089adba0439371565d9aa).
- Clean up [`eks/alb` policy creation](https://github.com/aws/aws-k8s-tester/commit/5ec5c23b8d8b8d01590112617a92ed1648bf227b).
  - Do not create `eks/alb` policy.
- Implement [after commands](https://github.com/aws/aws-k8s-tester/commit/52f885f4a0e14bf162afd5cd5b46cdf50247ede3).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.12`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.12) to [`v1.29.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.14).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.6](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.6) (2020-02-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.5...v0.6.6).

### `eks`

- Fix [VPC creation](https://github.com/aws/aws-k8s-tester/commit/5c4117ae1a368b57982fc7b8de94fb8009fd0266).
- Delete [`eks/metrics`, fix health check with metrics](https://github.com/aws/aws-k8s-tester/commit/).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.5) (2020-02-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.4...v0.6.5).

### `eksconfig`

- Support [existing roles for `Parameters.RoleARN` and `AddOnManagedNodeGroups.RoleARN`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
  - See [`issues#80`](https://github.com/aws/aws-k8s-tester/issues/80).
- Change [`Status.ClusterStatus` type from `string` to `[]ClusterStatus`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Status.ClusterStatusCurrent`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Config.RecordStatus`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Fix [`PrivateSubnetIDs` validation](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Parameters.VPCCreate`](https://github.com/aws/aws-k8s-tester/commit/92fd5589061d619d39f0427387065ddbb4440ee8).
- Add [`Parameters.EncryptionCMKCreate`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Parameters.EncryptionCMKARN`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Status.EncryptionCMKARN`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).
- Add [`Status.EncryptionCMKID`](https://github.com/aws/aws-k8s-tester/commit/87b424593e28c8ea0b9d2b8ad6a6122247cae3dd).

### `eks`

- Fix [health check](https://github.com/aws/aws-k8s-tester/commit/f57be0119a066e3502f75ebc42a9a869a6d1254e).
- Improve [status tracking](https://github.com/aws/aws-k8s-tester/commit/f57be0119a066e3502f75ebc42a9a869a6d1254e).
- Fix [`eks/mng` delete operation](https://github.com/aws/aws-k8s-tester/commit/d9bfa9b7fbcf2063a81d161f832755528318c204).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.6.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.4) (2020-02-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.3...v0.6.4).

### `eks`

- Fix [VPC public subnet creation](https://github.com/aws/aws-k8s-tester/commit/672b3e13bbd10273a6f88e524eee1c6042a5a789).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


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

