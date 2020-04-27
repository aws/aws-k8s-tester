

<hr>


## [v0.8.8](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.8) (2020-03-21)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.7...v0.8.8).

### `eks`

- Fix [MNG security group creation](https://github.com/aws/aws-k8s-tester/commit/210f3b91f7c953be08b46dce6e9e7f8e1cfdde03).
- Improve [MNG debugging](https://github.com/aws/aws-k8s-tester/commit/210f3b91f7c953be08b46dce6e9e7f8e1cfdde03).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.7](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.7) (2020-03-21)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.6...v0.8.7).

### `eks`

- Implement [upstream `kubetest2` interface](https://github.com/aws/aws-k8s-tester/commit/210f3b91f7c953be08b46dce6e9e7f8e1cfdde03).
  - https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Deployer
  - https://pkg.go.dev/k8s.io/test-infra/kubetest2/pkg/types?tab=doc#Options

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.6](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.6) (2020-03-21)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.5...v0.8.6).

### `eks`

- Improve [error handling in downloading binaries](https://github.com/aws/aws-k8s-tester/commit/5522d9e36c172d644bd9c0227dfe639511fba8ee).

### `pkg/github`

- Initial [commit](https://github.com/aws/aws-k8s-tester/commit/5522d9e36c172d644bd9c0227dfe639511fba8ee).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.28`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.28) to [`v1.29.29`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.29).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.5](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.5) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.4...v0.8.5).

### `ec2config`

- Fix [`RemoteAccessPrivateKeyPath` if it exists](https://github.com/aws/aws-k8s-tester/commit/de80c0d62dd07be980d487e9631f33377c5a27c1).

### `eksconfig`

- Fix [`RemoteAccessPrivateKeyPath` if it exists](https://github.com/aws/aws-k8s-tester/commit/de80c0d62dd07be980d487e9631f33377c5a27c1).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.4](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.4) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.3...v0.8.4).

### `ec2`

- Update [default SSM doc parameters](https://github.com/aws/aws-k8s-tester/commit/bafdba80410d4d5c48c292518cf09f4872bb9c6f).

### `eks`

- Update [default SSM doc parameters](https://github.com/aws/aws-k8s-tester/commit/bafdba80410d4d5c48c292518cf09f4872bb9c6f).
- Remove [remove "Error" level logging](https://github.com/aws/aws-k8s-tester/commit/6204c255ee9587cdd6f940ddd5c0f64c3c03ffc1).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.3](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.3) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.2...v0.8.3).

### `eksconfig`

- Change [default `RemoteAccessKeyName` suffixed by `-key-nodes`](https://github.com/aws/aws-k8s-tester/commit/92292eb2f3159a9ebb37af134a86ec66a67de26b).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.2) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.1...v0.8.2).

### `ec2`

- Update [S3 test file uploads](https://github.com/aws/aws-k8s-tester/commit/167fcfab94e095714809c970bb77c1789e8b2d69).
- Log [logs gzipped file size](https://github.com/aws/aws-k8s-tester/commit/d7adefe366ea4975f1445882f6df2be13b44dc5b).

### `eks`

- Update [S3 test file uploads](https://github.com/aws/aws-k8s-tester/commit/167fcfab94e095714809c970bb77c1789e8b2d69).
- Log [logs gzipped file size](https://github.com/aws/aws-k8s-tester/commit/d7adefe366ea4975f1445882f6df2be13b44dc5b).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>



## [v0.8.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.1) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.0...v0.8.1).

### `eks`

- Fix [nil pointer panic on sub-testers when `AddOnManagedNodeGroups.Enable` is `false`](https://github.com/aws/aws-k8s-tester/commit/0a28f7c3ed98b4ddbaed2a760057011ef42416b2).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v0.8.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.8.0) (2020-03-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.8...v0.8.0).

### `ec2`

- Upload [gzipped log files to S3 bucket](https://github.com/aws/aws-k8s-tester/commit/7290e32e56920eba9ed3cd29adbe076acfe71490).
  - `FetchLogs` is `true` and `S3BucketName` is non-empty, then a gzipped log file is uploaded to S3.

### `eks`

- Support [`AddOnNodeGroups` `SSMDocument*` for bottlerocket AMIs](https://github.com/aws/aws-k8s-tester/commit/5ddb73b26debb8858380a2c9f31c942f9537f0f8).
- Upload [gzipped log files to S3 bucket](https://github.com/aws/aws-k8s-tester/commit/7290e32e56920eba9ed3cd29adbe076acfe71490).
  - `FetchLogs` is `true` and `S3BucketName` is non-empty, then a gzipped log file is uploaded to S3.
- Fix [`SSHCommands`](https://github.com/aws/aws-k8s-tester/commit/c9841693c8b5efb70012630a7f2a0d5f21e9fdf6).

### `eksconfig`

- Support [`AddOnNodeGroups` `SSMDocument*` for bottlerocket AMIs](https://github.com/aws/aws-k8s-tester/commit/b7a37a18dcbe1f0ecbc519c92260e3def26e9135).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.27`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.27) to [`v1.29.28`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.28).

### Go

- Compile with [*Go 1.14.0*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

