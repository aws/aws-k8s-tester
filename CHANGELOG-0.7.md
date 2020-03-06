


<hr>


## [v0.7.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.7.1) (2020-03-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.7.0...v0.7.1).

### `eks`

- Remove [`TemplateVPCPublic` to only use VPC templates with public and private subnets](https://github.com/aws/aws-k8s-tester/commit/f445f1aac5055fbb06356a86638d3ff39f115ffe).
  - Auto-created VPCs will have both public and private subnets.

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

