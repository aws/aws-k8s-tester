

<hr>


## [v0.6.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.0) (2020-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.9...v0.6.0).

### `ec2config`

- Use [`;` for `Tags` and `IngressRulesTCP` as divider character when parsing `reflect.Map`](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### `eks`

- Test [`EKS` API availability at the beginning](https://github.com/aws/aws-k8s-tester/commit/6fd70924f4d86055ecef4f9596ecf08c4e772df3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Do [not fetch OIDC endpoints when a cluster is being deleted](https://github.com/aws/aws-k8s-tester/commit/8825e8865e934cda97c6cb65078d6b562ef17f68).
- Use [`github.com/aws/aws-sdk-go/service/eks.ClusterStatus*` for status checks](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).

### `eksconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Disable [`AddOnALB2048` by default](https://github.com/aws/aws-k8s-tester/commit/f437b006afbc304bd1552fa143cfcd6a5cbc8e39).
- Rename [`AddOnManagedNodeGroups.LogDir` to `AddOnManagedNodeGroups.LogsDir`](https://github.com/aws/aws-k8s-tester/commit/bf3a92a97fbe4571388f7909225129fe3ee926da).
- Improve [`AddOnManagedNodeGroups.LogsDir` defaults](https://github.com/aws/aws-k8s-tester/commit/4524c52ab907152bc85c656c54864e075f7ec5f3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).

### `kmsconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.1) to [`v1.29.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.2).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>

