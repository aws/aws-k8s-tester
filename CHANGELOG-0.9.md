

<hr>


## [v0.9.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.9.0) (2020-03-22)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.8.8...v0.9.0).

### `ec2config`

- Include [more fields from `*ec2.Instance` when `ConvertInstance`](https://github.com/aws/aws-k8s-tester/commit/4da03155db15ed1880da46a51c69db1ad04959dd).

### `eksconfig`

- Add [`MNG.ASGName` field](https://github.com/aws/aws-k8s-tester/commit/bc9f04c99baf1a4914bed4689fae308f536f247f).

### `eks`

- Improve [node group waits using EC2 Private DNS](https://github.com/aws/aws-k8s-tester/commit/eafc1f84c2096b07edcb501d1ddfa99f6c545d64).
- Run [`kubectl get nodes` while waiting for node groups](https://github.com/aws/aws-k8s-tester/commit/ed19ebf6b7abde641552273e35bd2f7a8a1d86fd).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

