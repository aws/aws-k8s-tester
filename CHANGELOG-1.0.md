

<hr>


## [v1.0.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.0) (2020-04)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.8...v1.0.0).

### `ec2config`

- Update [`README.md`](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Clean up [`RemoteAccessPrivateKeyPath` defaults](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).

### `eksconfig`

- Update [`README.md`](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Clean up [`RemoteAccessPrivateKeyPath` defaults](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).

### `eks`

- Add [missing `AddOnCSRs` delete operation](https://github.com/aws/aws-k8s-tester/commit/e91e12f256a60d74a9f08dead964608f74beee5a).
- Add [missing `AddOnConfigMaps` delete operation](https://github.com/aws/aws-k8s-tester/commit/e91e12f256a60d74a9f08dead964608f74beee5a).

### `pkg/k8s-client`

- Increase [`DefaultNamespaceDeletionInterval` from 5-second to 15-second](https://github.com/aws/aws-k8s-tester/commit/1a41c61813e1e0872b44738773ccdda4e765be1c).
- Improve [`DeleteNamespaceAndWait` retries on `i/o timeout`](https://github.com/aws/aws-k8s-tester/commit/1a41c61813e1e0872b44738773ccdda4e765be1c).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

