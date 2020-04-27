


<hr>


## [v0.4.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.4.4) (2019-11-07)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.4.3...v0.4.4).

### `aws-k8s-tester`

- Rename [`aws-k8s-tester eks list clusters --delete-failed` flag](https://github.com/aws/aws-k8s-tester/commit/3d5379d465cb29bac7519178ee25e11b48d6d02d).
- Add [`aws-k8s-tester eks list clusters --delete-prefix` flag](https://github.com/aws/aws-k8s-tester/commit/c0e2da2b910176b9f8dad59e35712689ec289c34).
- Remove [`aws-k8s-tester eks list clusters --more` flag](https://github.com/aws/aws-k8s-tester/commit/2d253ca97d9dfb5efb50459eaa4fdb75359d28ef).
- Remove [`aws-k8s-tester eks list clusters --clean-up` flag](https://github.com/aws/aws-k8s-tester/commit/2d253ca97d9dfb5efb50459eaa4fdb75359d28ef).
- Add [`aws-k8s-tester eks list clusters --delete-dry` flag](https://github.com/aws/aws-k8s-tester/commit/c2af645e9af5b06afa460eb944178b6f38333cab).

### `kmsconfig`

- Initial [commit](https://github.com/aws/aws-k8s-tester/commit/eab638497e57562046398784934c93b9c37dfb4b).

### `internal/kms`

- Initial [commit](https://github.com/aws/aws-k8s-tester/commit/eab638497e57562046398784934c93b9c37dfb4b).

### `internal/prow`

- [Remove](https://github.com/aws/aws-k8s-tester/commit/050ed364929cfe15b638e3b2d9a3f9b1b0e197d9).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.25.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.1) to [`v1.25.30`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.30).
- Upgrade [`go.uber.org/zap`](https://github.com/uber-go/zap/releases) from [`v1.10.0`](https://github.com/uber-go/zap/releases/tag/v1.10.0) to [`v1.12.0`](https://github.com/uber-go/zap/releases/tag/v1.12.0).
- Pin [`k8s.io/*` dependencies](https://github.com/aws/aws-k8s-tester/commit/2f1fdf5c7e47c4a001a3913e37c88b0c94a31f6a).

### Go

- Compile with [*Go 1.13.3*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.4.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.4.3) (2019-09-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.4.2...v0.4.3).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks list clusters --signing-name` flag](https://github.com/aws/aws-k8s-tester/commit/e4c8b5beb852e9b7c18cb17bcd6f6fbf85f7b2da).
- Add [`aws-k8s-tester eks list clusters --more` flag](https://github.com/aws/aws-k8s-tester/commit/e4c8b5beb852e9b7c18cb17bcd6f6fbf85f7b2da).
- Add [`aws-k8s-tester eks list clusters --clean-up` flag](https://github.com/aws/aws-k8s-tester/commit/e4c8b5beb852e9b7c18cb17bcd6f6fbf85f7b2da).

### `eksconfig`

- Add [`EKSTags` field](https://github.com/aws/aws-k8s-tester/commit/954468626d8e2bab3010af8bef2791de03ef5263).
- Use [airport code and region for tagging](https://github.com/aws/aws-k8s-tester/commit/74991b3050073e063a9d34bd99a60cfcfd715cfe).

### `awsapi`

- Add [airport code utilities](https://github.com/aws/aws-k8s-tester/commit/74991b3050073e063a9d34bd99a60cfcfd715cfe).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.24.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.24.3) to [`v1.25.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.25.1).

### Go

- Compile with [*Go 1.13.1*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.4.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.4.2) (2019-09-23)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.4.1...v0.4.2).

### `eksconfig`

- Add [`WorkerNodeCFTemplatePath` field](https://github.com/aws/aws-k8s-tester/commit/e33beb235c86420a693a367a39a7a810580bd475).
- Add [`WorkerNodeCFTemplateAdditionalParameterKeys` field](https://github.com/aws/aws-k8s-tester/commit/e33beb235c86420a693a367a39a7a810580bd475).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.24.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.24.2) to [`v1.24.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.24.3).

### Go

- Compile with [*Go 1.13.0*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.4.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.4.1) (2019-09-21)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.4.0...v0.4.1).

### `eksconfig`

- Add [`EKSRequestHeader` field](https://github.com/aws/aws-k8s-tester/commit/ecaa236b66967b1aaff8b938e3daeb4ed0a59df8).
- Add [`EKSSigningName` field](https://github.com/aws/aws-k8s-tester/commit/ecaa236b66967b1aaff8b938e3daeb4ed0a59df8).

### Go

- Compile with [*Go 1.13.0*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.4.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.4.0) (2019-09-20)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.3.4...v0.4.0).

### Release

- Prefix version string with `v` (e.g. not `0.4.0`, now use `v0.4.0`).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks list clusters`](https://github.com/aws/aws-k8s-tester/commit/09994664f2ef14d07f21b941dce5caa6c99272d0).
- Add [`aws-k8s-tester eks get worker-node-ami`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).
- Delete [`aws-k8s-tester ec2 create cluster --terminate-on-exit` flag](https://github.com/aws/aws-k8s-tester/commit/67aa1e2a55e48aa29bded1f60b533fde5fc1883f).
- Delete [`aws-k8s-tester eks create cluster --terminate-on-exit` flag](https://github.com/aws/aws-k8s-tester/commit/67aa1e2a55e48aa29bded1f60b533fde5fc1883f).

### `ec2config`

- Rename [`Down` field to `DestroyAfterCreate`](https://github.com/aws/aws-k8s-tester/commit/67aa1e2a55e48aa29bded1f60b533fde5fc1883f).
- Rename [`WaitBeforeDown` field to `DestroyWaitTime`](https://github.com/aws/aws-k8s-tester/commit/67aa1e2a55e48aa29bded1f60b533fde5fc1883f).

### `eks`

- Get [worker node AMI automatically through SSM parameter](https://github.com/aws/aws-k8s-tester/commit/e4a5e9439608955f756d3b37c68f897b71de7912).
  - More changes in [`git@d1f0800f2d`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).
- Add [`"Kind"` tag to VPC template](https://github.com/aws/aws-k8s-tester/commit/d81ea52a8f51f2bcd43daaaa64154a82f6f53c1b).
- Add [`"Creation"` tag to VPC template](https://github.com/aws/aws-k8s-tester/commit/f1b48ea59f72a64d950954b413ed45dc024c6593).

### `eksconfig`

- Rename [`EKSCustomEndpoint` field to `EKSResolverURL`](https://github.com/aws/aws-k8s-tester/commit/09994664f2ef14d07f21b941dce5caa6c99272d0).
- Rename [`WorkerNodeAMI` field to `WorkerNodeAMIID`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).
- Rename [`Down` field to `DestroyAfterCreate`](https://github.com/aws/aws-k8s-tester/commit/f0c94407ec7746677acf85e851dcd45313d7bae9).
- Rename [`WaitBeforeDown` field to `DestroyWaitTime`](https://github.com/aws/aws-k8s-tester/commit/f0c94407ec7746677acf85e851dcd45313d7bae9).
- Add [`WorkerNodeUserName` field](https://github.com/aws/aws-k8s-tester/commit/d56c5bd679c3d76bd33b288d95ecd3743ec6c27a).

### `e2e`

- Initial commit for [testing libraries](https://github.com/aws/aws-k8s-tester/tree/master/e2e).

### `pkg/cloud`

- Initial commit for [testing libraries](https://github.com/aws/aws-k8s-tester/tree/master/pkg/cloud).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.23.13`](https://github.com/aws/aws-sdk-go/releases/tag/v1.23.13) to [`v1.24.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.24.2).

### Go

- Compile with [*Go 1.13.0*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


