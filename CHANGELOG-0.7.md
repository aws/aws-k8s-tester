

<hr>


## [v0.7.9](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.9) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.8...v0.7.9).

### `eks`

- Support [`AddOnNodeGroups` `SSMDocument*` for bottlerocket AMIs](https://github.com/aws/aws-k8s-tester/commit/5ddb73b26debb8858380a2c9f31c942f9537f0f8).

### `eksconfig`

- Support [`AddOnNodeGroups` `SSMDocument*` for bottlerocket AMIs](https://github.com/aws/aws-k8s-tester/commit/b7a37a18dcbe1f0ecbc519c92260e3def26e9135).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.8](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.8) (2020-03-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.7...v0.7.8).

### `ec2`

- Fix [S3 clean-up when `S3BucketName` is empty](https://github.com/aws/aws-k8s-tester/commit/6b11852e3e32e5276f5beddbbe3322b6335d247b).

### `eks`

- Fix [S3 clean-up when `S3BucketName` is empty](https://github.com/aws/aws-k8s-tester/commit/6b11852e3e32e5276f5beddbbe3322b6335d247b).
- Fix [add-in install when `AddOn(Managed)NodeGroup` is `nil`](https://github.com/aws/aws-k8s-tester/commit/6b11852e3e32e5276f5beddbbe3322b6335d247b).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.7.7](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.7) (2020-03-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.6...v0.7.7).

### `ec2`

- Skip [private key S3 uploads if `S3BucketName` is empty](https://github.com/aws/aws-k8s-tester/commit/2823aa88f7455289a2aa34589c388177fa3e8507).

### `eks`

- Skip [private key S3 uploads if `S3BucketName` is empty](https://github.com/aws/aws-k8s-tester/commit/2823aa88f7455289a2aa34589c388177fa3e8507).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.26`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.26) to [`v1.29.27`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.27).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>



## [v0.7.6](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.6) (2020-03-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.5...v0.7.6).

### `ec2config`

- Make [S3 uploads optional](https://github.com/aws/aws-k8s-tester/commit/84d7e037fd8682981ea08bb9799c291f5376f377).

### `eksconfig`

- Make [S3 uploads optional](https://github.com/aws/aws-k8s-tester/commit/84d7e037fd8682981ea08bb9799c291f5376f377).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>



## [v0.7.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.5) (2020-03-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.4...v0.7.5).

### `aws-k8s-tester`

- Remove [`aws-k8s-tester eks create mng` sub-command](https://github.com/aws/aws-k8s-tester/commit/36ee870ba3b84954c771184fb3f7b8b582862d9b).

### `ec2config`

- Add [`ImageIDSSMParameter`](https://github.com/aws/aws-k8s-tester/commit/81c5af7df626bae2721e89a09ae05a061be15ceb).
- Add [`ASGsFetchLogs`](https://github.com/aws/aws-k8s-tester/commit/8ee120834bf615f7ec2e94cb2a1d973c88472eb6).
- Rename [`LogsDir` to `ASGsLogsDir`](https://github.com/aws/aws-k8s-tester/commit/8ee120834bf615f7ec2e94cb2a1d973c88472eb6).
- Add [`ASG.InstallSSM` field](https://github.com/aws/aws-k8s-tester/commit/c3f431d325b26784acd9b33d66bd73605e6c1efb).
- Add [`ASG.SSMDocument*` field](https://github.com/aws/aws-k8s-tester/commit/5df6dd4299d09ab794fb025230a90e3efc67ade8).
- Add [`S3Bucket*` fields](https://github.com/aws/aws-k8s-tester/commit/5df6dd4299d09ab794fb025230a90e3efc67ade8).
- Add [`Instance.RemoteAccessUserName` field](https://github.com/aws/aws-k8s-tester/commit/12441b3af9a2eba207ca3ae4ce7d07ef6844c7ac).

### `ec2`

- Clean up [terminal outputs, no color string](https://github.com/aws/aws-k8s-tester/commit/94e0a8b9e019c935c113f8e27274d8790490abec).
- Fix [custom AMI ASG CFN template](https://github.com/aws/aws-k8s-tester/commit/81c5af7df626bae2721e89a09ae05a061be15ceb).
- Fix [key creation](https://github.com/aws/aws-k8s-tester/commit/61487a22279956c2575affaf1c97896474ce475e).
- Create [ASG and launch configuration in a separate CFN stack](https://github.com/aws/aws-k8s-tester/commit/c3f431d325b26784acd9b33d66bd73605e6c1efb).
- Support [`bottlerocket` AMIs](https://github.com/aws/aws-k8s-tester/commit/dfd759622b5c3092e0ff7d4c00a636386addce70).
  - See https://github.com/bottlerocket-os/bottlerocket.
  - Configurable [EC2 metadata and user data](https://github.com/aws/aws-k8s-tester/commit/410d5b491d5cedcc763f689f6ef09d5c786be340).
- Support [SSM document command](https://github.com/aws/aws-k8s-tester/commit/b72c06301f61bc9424baae34ae59b1ebbac1e44c).
  - Require [S3 managed policy in IAM role for SSM output uploads](https://github.com/aws/aws-k8s-tester/commit/000766975c8636390fa88ed95c41522b7f8c9247).
- Support [S3 bucket creation](https://github.com/aws/aws-k8s-tester/commit/b72c06301f61bc9424baae34ae59b1ebbac1e44c).
- Upload [test artifacts to S3 bucket](https://github.com/aws/aws-k8s-tester/commit/372aeb1ac12566f5213667133c1bdc7b85926487).
  - Require [S3 managed policy in IAM role for SSM output uploads](https://github.com/aws/aws-k8s-tester/commit/000766975c8636390fa88ed95c41522b7f8c9247).

### `eksconfig`

- Move [`AddOnManagedNodeGroups.RemoteAccessKeyCreate` to `Config.RemoteAccessKeyCreate`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_KEY_CREATE` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE`.
- Move [`AddOnManagedNodeGroups.RemoteAccessKeyName` to `Config.RemoteAccessKeyName`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_KEY_NAME` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME`.
- Move [`AddOnManagedNodeGroups.RemoteAccessPrivateKeyPath` to `Config.RemoteAccessPrivateKeyPath`](https://github.com/aws/aws-k8s-tester/commit/0179c3a94106e82388158f7efd07d951d55023d3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_PRIVATE_KEY_PATH` is now `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH`.
- Move [`AddOnManagedNodeGroups.RemoteAccessUserName` to `MNG.RemoteAccessUserName`](https://github.com/aws/aws-k8s-tester/commit/12441b3af9a2eba207ca3ae4ce7d07ef6844c7ac).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_REMOTE_ACCESS_USER_NAME` is now deprecated.
  - Instead, use `remote-access-user-name` in `MNG` struct.
  - e.g. `{"mng-test-name-cpu":{"name":"mng-test-name-cpu","tags":{"cpu":"hello-world"},"remote-access-user-name":"ec2-user","release-version":"test-ver-cpu","ami-type":"AL2_x86_64","asg-min-size":17,"asg-max-size":99,"asg-desired-capacity":77,"instance-types":["type-cpu-1","type-cpu-2"],"volume-size":40},"mng-test-name-gpu":{"name":"mng-test-name-gpu","remote-access-user-name":"ec2-user","tags":{"gpu":"hello-world"},"release-version":"test-ver-gpu","ami-type":"AL2_x86_64_GPU","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"instance-types":["type-gpu-1","type-gpu-2"],"volume-size":500}}`
- Add [`AddOnManagedNodeGroups.FetchLogs` to configure fetch managed node group logs downloading](https://github.com/aws/aws-k8s-tester/commit/d57a203315b842bea6cab7476a778624155fdee3).
  - `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS` is `true` by default.
  - `FetchLogs` will be [skipped if `AddOnManagedNodeGroups.FetchLogs` is `false`](https://github.com/aws/aws-k8s-tester/commit/74aee02ac01123cbc8036910831addbca665cbbc).
- Add [`AddOnAppMesh`](https://github.com/aws/aws-k8s-tester/pull/81).
  - Enable AppMesh add-on with `AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE=true`.
- Add [`S3Bucket*` fields](https://github.com/aws/aws-k8s-tester/commit/f9c2ee1c2f9d4c6fc38e84c950f11616b3402713).
- Remove [`AddOnIRSA.S3BucketName` field](https://github.com/aws/aws-k8s-tester/commit/f9c2ee1c2f9d4c6fc38e84c950f11616b3402713).
- Support [custom worker node AMI](https://github.com/aws/aws-k8s-tester/commit/36ee870ba3b84954c771184fb3f7b8b582862d9b).
- Remove [`StatusManagedNodeGroups`](https://github.com/aws/aws-k8s-tester/commit/36ee870ba3b84954c771184fb3f7b8b582862d9b).
- Remove [`StatusManagedNodeGroup`](https://github.com/aws/aws-k8s-tester/commit/36ee870ba3b84954c771184fb3f7b8b582862d9b).

### `eks`

- Clean up [terminal outputs, no color string](https://github.com/aws/aws-k8s-tester/commit/94e0a8b9e019c935c113f8e27274d8790490abec).
- Add [`eks/appmesh`](https://github.com/aws/aws-k8s-tester/pull/81).
- Set [`EKS 1.15` as default](https://github.com/aws/aws-k8s-tester/commit/49d364c710b87ee5bcd6f22684c0de861ae3f86e).
- Move [remote access key creation/deletion function from `eks/mng` to `eks`](https://github.com/aws/aws-k8s-tester/commit/d110238d6ba93300d3109f2925bcc6a5cd254ad0).
- Remove [unused IAM policy creation/deletion function](https://github.com/aws/aws-k8s-tester/commit/21ea5769a46b9a2ecd5cac041570d4bc1d1d62d1).
- Fix [key creation](https://github.com/aws/aws-k8s-tester/commit/61487a22279956c2575affaf1c97896474ce475e).
- Support [S3 bucket creation](https://github.com/aws/aws-k8s-tester/commit/dc906341d1254cd2d89588f388efbe93c0a53c3d).
- Upload [test artifacts to S3 bucket](https://github.com/aws/aws-k8s-tester/commit/b8467a95f28131efe859c5580e5e89d9639a50a3).
  - Require [S3 managed policy in IAM role](https://github.com/aws/aws-k8s-tester/commit/000766975c8636390fa88ed95c41522b7f8c9247).
- Support [custom worker node AMI](https://github.com/aws/aws-k8s-tester/commit/36ee870ba3b84954c771184fb3f7b8b582862d9b).
  - See [`24661b9f7` for the initial commit](https://github.com/aws/aws-k8s-tester/commit/24661b9f78897d3f360c7b5b033bb7365bb8c1f3).

### Dependency

- Clean up [`k8s.io/client-go` vendoring](https://github.com/aws/aws-k8s-tester/pull/81).
- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.20`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.20) to [`v1.29.26`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.26).
- Upgrade [`github.com/uber-go/zap`](https://github.com/uber-go/zap/releases) from [`v1.14.0`](https://github.com/uber-go/zap/releases/tag/v1.14.0) to [`v1.14.1`](https://github.com/uber-go/zap/releases/tag/v1.14.1).

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

