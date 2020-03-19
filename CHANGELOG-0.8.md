

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

