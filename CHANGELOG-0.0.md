

## [0.0.1](https://github.com/coreos/etcd/releases/tag/0.0.1) (TBD)

See [code changes](https://github.com/coreos/etcd/compare/0.0.0...0.0.1).

### `eksconfig`

- Add [`KubetestVerbose`](https://github.com/aws/awstester/commit/17189259558110b066a974f6ee6fb2b8242c03d5) field.
- Add [`KubetestControlTimeout`](https://github.com/aws/awstester/commit/17189259558110b066a974f6ee6fb2b8242c03d5) field.

### `awstester` CLI

- Add [`awstester version`](https://github.com/aws/awstester/commit/6d72c67fa1ae173fe211feb5d08aeaf596a7110e) command.
- Add ALB target health check to [`awstester eks test alb correctness`](https://github.com/aws/awstester/commit/152bb09d45b79d418e9069fbf86d3452fd027589).
  - Implemented in package `internal/eks/alb`.

### Depency

- Upgrade [`github.com/aws/aws-sdk-go`]() from [`v1.15.54`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.54) to [`v1.15.57`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.57).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


## [0.0.0](https://github.com/coreos/etcd/releases/tag/0.0.0) (2018-10-15)

Initial release.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).

