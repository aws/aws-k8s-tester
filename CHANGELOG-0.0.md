

## [0.0.2](https://github.com/aws/awstester/releases/tag/0.0.2) (2018-10-22)

See [code changes](https://github.com/aws/awstester/compare/0.0.1...0.0.2).

### `eksconfig`

- Add [`eksconfig.ClusterState.WorkerNodes`](https://github.com/aws/awstester/pull/5) field.
  - Add [`eksconfig.Instance`](https://github.com/aws/awstester/pull/5) to represent worker node.
- Add [`eksconfig.ClusterState.WorkerNodeLogs`](https://github.com/aws/awstester/pull/5) field.
- Add [`eksconfig.ClusterState.CFStackWorkerNodeGroupSecurityGroupID`](https://github.com/aws/awstester/pull/5) field.
- Add [`eksconfig.Config.EnableNodeSSH`](https://github.com/aws/awstester/pull/5) field.
  - If true, worker node exposes port :22 for SSH access.
- Rename [`eksconfig.ClusterState.EC2NodeGroupStatus` to `eksconfig.ClusterState.WorkerNodeGroupStatus`](https://github.com/aws/awstester/pull/5).
- Rename [`eksconfig.ClusterState.CFStackNode*` to `eksconfig.ClusterState.CFStackWorkerNode*`](https://github.com/aws/awstester/pull/5).
- Remove [`KubetestEnableDumpClusterLogs`](https://github.com/aws/awstester/pull/5) field.

### `eksdeployer`

- Implement [`DumpClusterLogs`](https://github.com/aws/awstester/pull/5) to export worker node logs.

### `internal`

- Implement worker node log exporter in [`internal/eks`](https://github.com/aws/awstester/pull/5).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.57`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.57) to [`v1.15.59`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.59).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


## [0.0.1](https://github.com/aws/awstester/releases/tag/0.0.1) (2018-10-18)

See [code changes](https://github.com/aws/awstester/compare/0.0.0...0.0.1).

### `eksconfig`

- Add [`KubetestVerbose`](https://github.com/aws/awstester/commit/17189259558110b066a974f6ee6fb2b8242c03d5) field.
- Add [`KubetestControlTimeout`](https://github.com/aws/awstester/commit/17189259558110b066a974f6ee6fb2b8242c03d5) field.
- Add [`KubetestEnableDumpClusterLogs`](https://github.com/aws/awstester/commit/aa4ab00bec7523bf154c0928727b62271758d93b) field.

### `eksdeployer`

- Add [`UploadToBucketForTests`](https://github.com/aws/awstester/commit/07e872a3b4b5758fc80d093d5a3b511b8bfe08f8) method.
- Remove [Publish](https://github.com/aws/awstester/commit/ad2a71dfc9687a9bd1a5869bd09a3f9eb771c504) method.
  - Not needed for now (to be added back later).
  - See [`kubetest/e2e.go`](https://github.com/kubernetes/test-infra/blob/fe0a9926c1c3d0a9d94e0d3c2f755dbdbc34d892/kubetest/e2e.go#L318-L322).

### `awstester` CLI

- Add [`awstester version`](https://github.com/aws/awstester/commit/6d72c67fa1ae173fe211feb5d08aeaf596a7110e) command.
- Add ALB target health check to [`awstester eks test alb correctness`](https://github.com/aws/awstester/commit/152bb09d45b79d418e9069fbf86d3452fd027589).
- Add presets to [`awstester eks prow alb`](https://github.com/aws/awstester/commit/6ed769cb9a0685e13a36e4dd83f14210a253b758) output.
  - Fix [issue#4](https://github.com/aws/awstester/issues/4).

### `internal`

- Implement [`internal/eks/alb` target health check test](https://github.com/aws/awstester/commit/152bb09d45b79d418e9069fbf86d3452fd027589).
- Fix [`/prow-status` refresh logic](https://github.com/aws/awstester/commit/ce495dc13c82bc9378de06648c559d90a5e28ce6).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.54`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.54) to [`v1.15.57`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.57).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


## [0.0.0](https://github.com/aws/awstester/releases/tag/0.0.0) (2018-10-15)

Initial release.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).

