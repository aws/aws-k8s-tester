

<hr>


## [v0.7.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.5) (2020-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.4...v0.7.5).

### `ec2config`

- Add [`ImageIDSSMParameter`](https://github.com/aws/aws-k8s-tester/commit/81c5af7df626bae2721e89a09ae05a061be15ceb).
- Add [`ASGsFetchLogs`](https://github.com/aws/aws-k8s-tester/commit/8ee120834bf615f7ec2e94cb2a1d973c88472eb6).
- Rename [`LogsDir` to `ASGsLogsDir`](https://github.com/aws/aws-k8s-tester/commit/8ee120834bf615f7ec2e94cb2a1d973c88472eb6).

### `ec2`

- Add [custom AMI ASG CFN template](https://github.com/aws/aws-k8s-tester/commit/81c5af7df626bae2721e89a09ae05a061be15ceb).
- Fix [key creation](https://github.com/aws/aws-k8s-tester/commit/61487a22279956c2575affaf1c97896474ce475e).

### `eksconfig`

- Move [`AddOnManagedNodeGroups.RemoteAccessKeyCreate` to `Config.RemoteAccessKeyCreate`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_KEY_CREATE` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE`.
- Move [`AddOnManagedNodeGroups.RemoteAccessKeyName` to `Config.RemoteAccessKeyName`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_KEY_NAME` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME`.
- Move [`AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath` to `Config.RemoteAccessPrivateKeyPath`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH`.
- Add [`AddOnManagedNodeGroups.FetchLogs` to configure fetch managed node group logs downloading](https://github.com/aws/aws-k8s-tester/commit/d57a203315b842bea6cab7476a778624155fdee3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS` is `true` by default.
  - `FetchLogs` will be [skipped if `AddOnManagedNodeGroups.FetchLogs` is `false`](https://github.com/aws/aws-k8s-tester/commit/74aee02ac01123cbc8036910831addbca665cbbc).
- Support [custom worker node AMI](https://github.com/aws/aws-k8s-tester/commit/).

### `eks`

- Set [`EKS 1.15` as default](https://github.com/aws/aws-k8s-tester/commit/49d364c710b87ee5bcd6f22684c0de861ae3f86e).
- Support [custom worker node AMI](https://github.com/aws/aws-k8s-tester/commit/).
- Move [remote access key creation/deletion function from `eks/mng` to `eks`](https://github.com/aws/aws-k8s-tester/commit/d110238d6ba93300d3109f2925bcc6a5cd254ad0).
- Remove [unused IAM policy creation/deletion function](https://github.com/aws/aws-k8s-tester/commit/21ea5769a46b9a2ecd5cac041570d4bc1d1d62d1).
- Fix [key creation](https://github.com/aws/aws-k8s-tester/commit/61487a22279956c2575affaf1c97896474ce475e).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>



## [v0.7.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.4) (2020-03-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.3...v0.7.4).

### `eks`

- Remove [`AllowedValues` for eks version](https://github.com/aws/aws-k8s-tester/commit/0cb2d0a2736d66ddf711144d0b95da548c1eb65a).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.3) (2020-03-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.2...v0.7.3).

### `pkg/aws`

- Allow [session creation without env vars (e.g. instance IAM role)](https://github.com/aws/aws-k8s-tester/commit/5c3a18b7395d8bd90f5a837b3b97c6521ede02de).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.2) (2020-03-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.1...v0.7.2).

### `ec2`

- Tag [`AWS::EC2::EIP` with `'${AWS::StackName}-EIP*'` during VPC creation](https://github.com/aws/aws-k8s-tester/commit/26893f1d472004b22ecb09a67a2c2cab4c238786).

### `eksconfig`

- Rename [`AddOnJobPerl` to `AddOnJobPi`](https://github.com/aws/aws-k8s-tester/commit/c1ea05d9584805a64ba4bb37f864ff88ac3545f2).
  - `AWS_K8S_TESTER_EKS_ADD_ON_JOB_PERL_*` is now `AWS_K8S_TESTER_EKS_ADD_ON_JOB_PI_`.
- Reduce [`AddOnJobEcho` default `Parallels` and `Completes` values](https://github.com/aws/aws-k8s-tester/commit/3b9b9583ab6f0a294525ec5ca3a056ebf201f845).
- Add [`AddOnCronJob`](https://github.com/aws/aws-k8s-tester/commit/ce4819124972610a392b6055a30321a1a5b9169e).
- Rename [`AddOnJobEcho.Size` to `AddOnJobEcho.EchoSize`](https://github.com/aws/aws-k8s-tester/commit/fa3fa7b3b11fd33c8dc923b9dc629b00dbf15864).
  - `AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_SIZE` is now `AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ECHO_SIZE`.

### `eks`

- Rename [`eks/jobs` package to `jobs-echo` and `jobs-pi`](https://github.com/aws/aws-k8s-tester/commit/c1ea05d9584805a64ba4bb37f864ff88ac3545f2).
- Add [`eks/cronjobs`](https://github.com/aws/aws-k8s-tester/commit/730cd1f473486f3449281958c00000e74e342a4c).
- Tag [`AWS::EC2::EIP` with `'${AWS::StackName}-EIP*'` during VPC creation](https://github.com/aws/aws-k8s-tester/commit/26893f1d472004b22ecb09a67a2c2cab4c238786).

### `version`

- Add [`Version` function](https://github.com/aws/aws-k8s-tester/commit/d582a0ee4c1c15d4945ca9fcc801cd433034ee81).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.19`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.19) to [`v1.29.20`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.20).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.1) (2020-03-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.0...v0.7.1).

### `eks`

- Remove [`TemplateVPCPublic` to only use VPC templates with public and private subnets](https://github.com/aws/aws-k8s-tester/commit/f445f1aac5055fbb06356a86638d3ff39f115ffe).
  - Auto-created VPCs will have both public and private subnets.
- Include [ALB policy in default roles](https://github.com/aws/aws-k8s-tester/commit/5d1de5d17e38880a88336cfb9ff2e454e8bea226).

### `version`

- Tag [resources with `ReleaseVersion` with the tag key `aws-k8s-tester-version`](https://github.com/aws/aws-k8s-tester/commit/4b77f640e8bdd8abe4100778777e6d7df5ff1229).
- Set [default values at compile](https://github.com/aws/aws-k8s-tester/commit/5a3ec45b5230747adfda28d22434dcef6b45430e).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.18) to [`v1.29.19`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.19).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.0) (2020-03-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.9...v0.7.0).

### `ec2`

- Rename [`DownloadClusterLogs` to `DownloadLogs`](https://github.com/aws/aws-k8s-tester/commit/e3cf908519a5a75fd11cecfe81ab55d64ebddb2d).

### `version`

- Tag [resources with `ReleaseVersion`](https://github.com/aws/aws-k8s-tester/commit/65e486474617e9128ebf0ed51572dcdae0ac691a).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

