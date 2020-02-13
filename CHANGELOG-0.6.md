

<hr>


## [v0.6.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.0) (2020-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.9...v0.6.0).

### `ec2config`

- Use [`;` for `Tags` and `IngressRulesTCP` as divider character when parsing `reflect.Map`](https://github.com/aws/aws-k8s-tester/commit/).
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/).

### `eksconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/).
- Disable [`AddOnALB2048` by default](https://github.com/aws/aws-k8s-tester/commit/f437b006afbc304bd1552fa143cfcd6a5cbc8e39).
- Rename [`AddOnManagedNodeGroups.LogDir` to `AddOnManagedNodeGroups.LogsDir`](https://github.com/aws/aws-k8s-tester/commit/bf3a92a97fbe4571388f7909225129fe3ee926da).
- Improve [`AddOnManagedNodeGroups.LogsDir` defaults](https://github.com/aws/aws-k8s-tester/commit/4524c52ab907152bc85c656c54864e075f7ec5f3).

### `kmsconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.1) to [`TODO`](https://github.com/aws/aws-sdk-go/releases/tag/TODO).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>

