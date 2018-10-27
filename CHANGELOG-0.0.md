

<hr>


## [0.0.8](https://github.com/aws/awstester/releases/tag/0.0.8) (2018-10-27)

See [code changes](https://github.com/aws/awstester/compare/0.0.7...0.0.8).

### `awstester` CLI

- Remove [`awstester eks prow alb`](https://github.com/aws/awstester/commit/86e4db7e341681ece0e4b89b1f747024c157d2f3).

### `internal`

- Refactor [worker node log fetch operation in `internal/eks`](https://github.com/aws/awstester/commit/77d318a449e52b2f9550d2e3e7586181b117da36).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.0.7](https://github.com/aws/awstester/releases/tag/0.0.7) (2018-10-26)

See [code changes](https://github.com/aws/awstester/compare/0.0.6...0.0.7).

### `awstester` CLI

- Rename to [`awstester alb-log convert-to-csv`](https://github.com/aws/awstester/commit/0b57ef14f43e086038880a4711b9e86017b76b0c).
- Rename to [`awstester alb-log count-targets`](https://github.com/aws/awstester/commit/0b57ef14f43e086038880a4711b9e86017b76b0c).
- Add [`awstester alb-log merge-raw`](https://github.com/aws/awstester/commit/0b57ef14f43e086038880a4711b9e86017b76b0c).
- Rename to [`awstester csi test integration`](https://github.com/aws/awstester/commit/b9d077fe06e59c56859a5f7524b7ef7584f2511a).
- Rename to [`awstester wrk average-raw`](https://github.com/aws/awstester/commit/b79a4717d33046ce90cccf0c6138c78105a4ac3d).
- Rename to [`awstester wrk convert-to-csv`](https://github.com/aws/awstester/commit/b79a4717d33046ce90cccf0c6138c78105a4ac3d).
- Rename to [`awstester wrk merge-csv`](https://github.com/aws/awstester/commit/b79a4717d33046ce90cccf0c6138c78105a4ac3d).
- Rename to [`awstester wrk merge-raw`](https://github.com/aws/awstester/commit/b79a4717d33046ce90cccf0c6138c78105a4ac3d).

### `eksconfig`

- Rename to [`eksconfig.Config.UploadTesterLogs`](https://github.com/aws/awstester/commit/e88759aaaf12a217ab24ceae4288e1f2f084b6cf).
- Add [`eksconfig.Config.UploadWorkerNodeLogs`](https://github.com/aws/awstester/commit/e88759aaaf12a217ab24ceae4288e1f2f084b6cf).
- Add [`ALBIngressController.UploadALBTesterLogs`](https://github.com/aws/awstester/commit/e88759aaaf12a217ab24ceae4288e1f2f084b6cf).
- Add [`eksconfig.Config.EnableWorkerNodeHA`](https://github.com/aws/awstester/commit/9b3f220477725bd57df169ba6b253e6d0d63f59a) field.
- Add [`ALBIngressController.TestMetrics`](https://github.com/aws/awstester/commit/4e5b676a9be4db2b28084dfb75484f428e67dc59) field.
- Change [default `eksconfig.Config.ALBIngressController.TestServerRoutes` value to 1](https://github.com/aws/awstester/commit/0a4e1cf26735e5c09cb0c81be7a9c6ec757318f8).
- Do not require [`AWSTesterImage` when `ALBIngressController.TestMode` is `"ingress-test-server"`](https://github.com/aws/awstester/commit/d9403b196961ec2d473e11127670da411dd19050).

### `internal`

- Support ['install-docker-ubuntu' plugin in `internal/ec2`](https://github.com/aws/awstester/commit/89b2cbe3f2acde4731fe748289981a2d8dc195ff).
- Fix [security group check with CIDR in `internal/eks`](https://github.com/aws/awstester/commit/2341499d666be3aa0aadde40bd81f1ec3751481e).
- Fix [auto scaling group checks on worker nodes](https://github.com/aws/awstester/commit/588d2634dbb3af43046d988abc6a09b8264e84c8).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.62`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.62) to [`v1.15.64`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.64).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.0.6](https://github.com/aws/awstester/releases/tag/0.0.6) (2018-10-24)

See [code changes](https://github.com/aws/awstester/compare/0.0.5...0.0.6).

### `eksconfig`

- Add [`KubetestEmbeddedBinary`](https://github.com/aws/awstester/commit/fed39903638dba3a96e83dca6c3dcbe0687317d1) field.
- Remove [`KubetestVerbose`](https://github.com/aws/awstester/commit/6a7d73febaa3d962621031832774c698a2684b39) field.
- Remove [`KubetestControlTimeout`](https://github.com/aws/awstester/commit/6a7d73febaa3d962621031832774c698a2684b39) field.

### `internal`

- Refactor [`internal/ec2/config/plugins`](https://github.com/aws/awstester/commit/50a6d2999925220c23dac744942952ad2685b8aa).
- Make [`internal/ec2/config.Config.LogAutoUpload` false by default](https://github.com/aws/awstester/commit/31abeb2189951af17360a76bb47e811ac47617c5).
- `internal/ec2/config.Config.Plugins`, if not empty, always [overwrites `internal/ec2/config.Config.InitScript`](https://github.com/aws/awstester/commit/cd04fdbcd6fe583d8e3331d882c5c88b98698fee#diff-144560acd519ff2592df00db3e840fd0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.59`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.59) to [`v1.15.62`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.62).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.0.5](https://github.com/aws/awstester/releases/tag/0.0.5) (2018-10-23)

See [code changes](https://github.com/aws/awstester/compare/0.0.4...0.0.5).

### `awstester` CLI

- Add [`awstester csi test e2e --vpc-id` flag](https://github.com/aws/awstester/commit/0d38677bc80e9466d9903190009eb3e78bd6a825).

### `internal`

- Add retries [`internal/ssh`](https://github.com/aws/awstester/commit/c89001098bce7557bc9e15fc90fab0f340d6a146).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.0.4](https://github.com/aws/awstester/releases/tag/0.0.4) (2018-10-23)

See [code changes](https://github.com/aws/awstester/compare/0.0.3...0.0.4).

### `awstester` CLI

- Add [`awstester csi test e2e`](https://github.com/aws/awstester/commit/b8cc23d43f4cc0b1ffe8d98c15f43df9af33bbdd) command.

### `internal`

- Add `Envs` field to [`internal/ssh.Config`](https://github.com/aws/awstester/commit/0049fe0de6bf9ba009a813da74049cbb01758faf).
- Add `OpOption` to [`internal/ssh`](https://github.com/aws/awstester/commit/85e335337e6f814cd3327cfea3ce7a5784184026).
- Add [`internal/ec2/config.Plugins`](https://github.com/aws/awstester/commit/a00f5a0742ed26d266d4a0c7c6299bc817ea2d6f) to provision EC2 instances.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.0.3](https://github.com/aws/awstester/releases/tag/0.0.3) (2018-10-22)

See [code changes](https://github.com/aws/awstester/compare/0.0.2...0.0.3).

### `awstester` CLI

- Add [`awstester eks test get-worker-node-logs`](https://github.com/aws/awstester/pull/6) command.

### `eksdeployer`

- Add [`GetWorkerNodeLogs`](https://github.com/aws/awstester/pull/6) to download worker node logs.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


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


<hr>


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


<hr>


## [0.0.0](https://github.com/aws/awstester/releases/tag/0.0.0) (2018-10-15)

Initial release.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

