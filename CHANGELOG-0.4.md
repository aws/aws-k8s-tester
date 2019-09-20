

<hr>


## [0.4.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.4.0) (2019-09 TBD)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.3.4...0.4.0).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks list clusters`](https://github.com/aws/aws-k8s-tester/commit/09994664f2ef14d07f21b941dce5caa6c99272d0).
- Add [`aws-k8s-tester eks get worker-node-ami`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).

### `eks`

- Get [worker node AMI automatically through SSM parameter](https://github.com/aws/aws-k8s-tester/commit/e4a5e9439608955f756d3b37c68f897b71de7912).
  - More changes in [`git@d1f0800f2d`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).
- Add [`"Kind"` tag to VPC template](https://github.com/aws/aws-k8s-tester/commit/d81ea52a8f51f2bcd43daaaa64154a82f6f53c1b).
- Add [`"Creation"` tag to VPC template](https://github.com/aws/aws-k8s-tester/commit/f1b48ea59f72a64d950954b413ed45dc024c6593).

### `eksconfig`

- Rename [`EKSCustomEndpoint` field to `EKSResolverURL`](https://github.com/aws/aws-k8s-tester/commit/09994664f2ef14d07f21b941dce5caa6c99272d0).
- Rename [`WorkerNodeAMI` field to `WorkerNodeAMIID`](https://github.com/aws/aws-k8s-tester/commit/d1f0800f2df575e9662fec15fb47a4080ee6664a).

### `pkg/cloud`

- Initial commit for [testing libraries](https://github.com/aws/aws-k8s-tester/tree/master/pkg/cloud).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.23.13`](https://github.com/aws/aws-sdk-go/releases/tag/v1.23.13) to [`v1.24.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.24.2).

### Go

- Compile with [*Go 1.13.0*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


