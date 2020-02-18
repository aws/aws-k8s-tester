


<hr>


## [v0.6.2](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.2) (2020-02-18)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.1...v0.6.2).

### `eksconfig`

- Set [`AddOnManagedNodeGroups.Enable` to `false` by default](https://github.com/aws/aws-k8s-tester/commit/865dcaeecd2b447a9322c38c908e359b466d0471).
- Set [`AddOnNLBHelloWorld.Enable` to `false` by default](https://github.com/aws/aws-k8s-tester/commit/c1400268aa7f2bcfccda94c8ecbc79c8f17239f7).
- Improve [`AddOnManagedNodeGroups.RoleServicePrincipals` validation](https://github.com/aws/aws-k8s-tester/commit/ac2ba073a223c683eb550c6734925eb9e10e1905).
- Add [`Parameters.VPCID` for VPC reuse](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Remove [`Parameters.PrivateSubnetIDs`](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Remove [`Parameters.ControlPlaneSecurityGroupID`](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).

### `eks`

- Support [existing VPC for cluster creation](https://github.com/aws/aws-k8s-tester/commit/78867cebf9ff6c2ff87b50d93dc6582d93373b49).
- Set [secret write fail threshold for `AddOnSecrets`](https://github.com/aws/aws-k8s-tester/commit/).
  - 10 consecutive `Secret` write failures returns an error.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.3) to [`v1.29.4`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.4).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>


## [v0.6.1](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.1) (2020-02-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.6.0...v0.6.1).

### `eks`

- Improve [failure logging](https://github.com/aws/aws-k8s-tester/commit/6604cbad3d64d885b16ce3246f78e3f5bc2cbc30).

### `eksconfig`

- Update [default `KubectlPath` value](https://github.com/aws/aws-k8s-tester/commit/95e8ed790e588a8f31758d901b2f8997b04d846f).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>



## [v0.6.0](https://github.com/aws/aws-k8s-tester/releases/tag/v0.6.0) (2020-02-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.5.9...v0.6.0).

### `ec2config`

- Use [`;` for `Tags` and `IngressRulesTCP` as divider character when parsing `reflect.Map`](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### `eks`

- Test [`EKS` API availability at the beginning](https://github.com/aws/aws-k8s-tester/commit/6fd70924f4d86055ecef4f9596ecf08c4e772df3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Do [not fetch OIDC endpoints when a cluster is being deleted](https://github.com/aws/aws-k8s-tester/commit/8825e8865e934cda97c6cb65078d6b562ef17f68).
- Use [`github.com/aws/aws-sdk-go/service/eks.ClusterStatus*` for status checks](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Improve [cluster status checks with `github.com/aws/aws-sdk-go/service/eks`](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [cluster status polling](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [`mng` status checks with `github.com/aws/aws-sdk-go/service/eks`](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).
- Improve [`mng` status polling](https://github.com/aws/aws-k8s-tester/commit/bd914082ecb6d2f84bf74184f24b2a174ae5d0b6).

### `eksconfig`

- Set [initial `eksconfig.Config.Name` in `eksconfig.NewDefault` using `AWS_K8S_TESTER_EKS_NAME` (if defined)](https://github.com/aws/aws-k8s-tester/commit/11c1fa3aaa654333069d002ecf1dc1e765deca02).
  - e.g. `AWS_K8S_TESTER_EKS_NAME=${USER}-cluster aws-k8s-tester eks create config -p test.yaml`
- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).
- Disable [`AddOnALB2048` by default](https://github.com/aws/aws-k8s-tester/commit/f437b006afbc304bd1552fa143cfcd6a5cbc8e39).
- Rename [`AddOnManagedNodeGroups.LogDir` to `AddOnManagedNodeGroups.LogsDir`](https://github.com/aws/aws-k8s-tester/commit/bf3a92a97fbe4571388f7909225129fe3ee926da).
- Improve [`AddOnManagedNodeGroups.LogsDir` defaults](https://github.com/aws/aws-k8s-tester/commit/4524c52ab907152bc85c656c54864e075f7ec5f3).
- Fix [cluster deletion when created via `EKS` API instead of CloudFormation](https://github.com/aws/aws-k8s-tester/commit/475bf253b0355a845e052dd3d383c8ccf072f749).
- Fix [`VPCID` checks](https://github.com/aws/aws-k8s-tester/commit/1758b1af46b71a837653518884414619e7003550).

### `kmsconfig`

- Remove [redundant environmental variable parsing methods](https://github.com/aws/aws-k8s-tester/commit/7ea5e64a2f3618fd48c62c25acdceff6d97677f0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.29.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.1) to [`v1.29.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.29.3).

### Go

- Compile with [*Go 1.13.8*](https://golang.org/doc/devel/release.html#go1.13).


<hr>

