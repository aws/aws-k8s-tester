


<hr>


## [v0.7.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.2) (2020-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.1...v0.7.2).

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

### `version`

- Add [`Version` function](https://github.com/aws/aws-k8s-tester/commit/d582a0ee4c1c15d4945ca9fcc801cd433034ee81).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.19`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.19) to [`vTODO`](https://github.com/aws/aws-sdk-go/releases/tag/vTODO).

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

