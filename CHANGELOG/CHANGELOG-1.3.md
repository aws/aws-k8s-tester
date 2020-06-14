

<hr>


## [v1.3.7](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.7) (2020-06-13)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.6...v1.3.7).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks --auto-path` to automatically create configuration file path](https://github.com/aws/aws-k8s-tester/commit/e824a43f267830ecb16b2266daf060caa9ab5d61).

### `eksconfig`

- Add [`AddOnMetricsServer`](https://github.com/aws/aws-k8s-tester/commit/17451bade7e3cab783f9839db3fca1df0825b4fd).
  - Set via `AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_ENABLE=true`.
  - Now `AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE=true` requires `AWS_K8S_TESTER_EKS_ADD_ON_METRICS_SERVER_ENABLE=true`.
- Change [`Config.RemoteAccessPrivateKeyPath` from `filepath.Join(homedir.HomeDir(), ".ssh", "kube_aws_rsa")` to `filepath.Join(os.TempDir(), randutil.String(15)+".insecure.key")`](https://github.com/aws/aws-k8s-tester/commit/a1ca992fb27fee6a58f61f445f3169badfc13638).
  - Can be set via `AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH`.
  - May break upstream tests (to be in sync with https://pkg.go.dev/k8s.io/kubernetes/test/e2e/framework#GetSigner).
  - Randomize to help create multiple clusters locally.
- Add [`Config.TotalHollowNodes`](https://github.com/aws/aws-k8s-tester/commit/b902ecac6a8fe95b3f1b6268db58fbe950616b2f).

### `eks`

- Improve [polling operations for `eks/cron-jobs`, `eks/jobs-echo`, and `eks/jobs-pi` with pagination](https://github.com/aws/aws-k8s-tester/commit/7306c6e0979541c8461388ffd32938c384e06e31).
- Update [default `sonobuoy` version to `v0.18.3` in `eks/conformance`](https://github.com/vmware-tanzu/sonobuoy/releases/tag/v0.18.3).
- Improve [`eks/kubernetes-dashboard` `metrics-server` `kubectl top node` logging](https://github.com/aws/aws-k8s-tester/commit/b0889909a225497265a2fced0063bd51b813108c).
- Add [sleep wait after deleting deployment objects](https://github.com/aws/aws-k8s-tester/commit/0f65f550a11b146d775dccaf867e6e3872da0894).
- Rename [`eks/mng/node-waiter` to `eks/mng/wait`](https://github.com/aws/aws-k8s-tester/commit/6bed26a3eed0d73b47a1d516a17a8169d95f91e9).
- Rename [`eks/ng/node-waiter` to `eks/ng/wait`](https://github.com/aws/aws-k8s-tester/commit/6bed26a3eed0d73b47a1d516a17a8169d95f91e9).
- Increase [cluster create/delete timeout from 30-min to 1-hour](https://github.com/aws/aws-k8s-tester/commit/10523bb628162f1b2b1f1f4455efa0fa88564752).
- Move [`eks/metrics-server` from `eks/kubernetes-dashboard`](https://github.com/aws/aws-k8s-tester/commit/17451bade7e3cab783f9839db3fca1df0825b4fd).
  - Fix [`kubectl top node`](https://github.com/aws/aws-k8s-tester/commit/ec76fedc824d48aa31fbc39c29ff7dc1d0bbfe27).
- Fix [`eks/fargate` delete profile polling operation](https://github.com/aws/aws-k8s-tester/commit/b32f65b38738a3563b1d35ac058bc6a44f338aac).
- Move [`eks/fargate.Poll` to `eks/fargate/wait`](https://github.com/aws/aws-k8s-tester/commit/72ec6d81f1475acfef2e8561b8a62f35fad3d0df).
- Improve [`eks/ng` and `eks/mng` deletion by pre-delete lingering ENIs](https://github.com/aws/aws-k8s-tester/commit/da879fcd5f20210f4401193103b996339fb97ca4).

### `pkg/k8s-client`

- Increase [default namespace delete timeout from 15-min to 30-min](https://github.com/aws/aws-k8s-tester/commit/0f65f550a11b146d775dccaf867e6e3872da0894).

### `ssh`

- Improve [retries, do not retry on non-zero command run exit codes](https://github.com/aws/aws-k8s-tester/commit/cb353464f24ed0607e8accd7f99fc68a46322a01).
- Log [connection close with `Debug` level](https://github.com/aws/aws-k8s-tester/commit/7e8ce119986b682185d8e1e93dfdbaf9d3ed89cc).

### Dependency

- Update [default `sonobuoy` version to `v0.18.3`](https://github.com/vmware-tanzu/sonobuoy/releases/tag/v0.18.3).


<hr>


## [v1.3.6](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.6) (2020-06-12)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.5...v1.3.6).

### `ec2config`

- Add [`Config.RoleCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/13922f4a0eb9446705a757c816923ae54a03bb41).
- Add [`Config.VPCCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/13922f4a0eb9446705a757c816923ae54a03bb41).
- Add [`ASG.ASGCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/13922f4a0eb9446705a757c816923ae54a03bb41).
- Add [`ASG.SSMDocumentCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/13922f4a0eb9446705a757c816923ae54a03bb41).

### `ec2`

- Improve [s3 upload error message](https://github.com/aws/aws-k8s-tester/commit/e29f3038135a1e43e4e5d201ad538ad3973a0c6a).

### `eksconfig`

- Add [`Parameters.RoleCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/db0cb5d39e3b1d9758f31ca4f5425ad9d1f711ce).
- Add [`Parameters.VPCCFNStackYAMLFilePath`](https://github.com/aws/aws-k8s-tester/commit/db0cb5d39e3b1d9758f31ca4f5425ad9d1f711ce).
- Add [`MNGVersionUpgrade` for managed node group version upgrades](https://github.com/aws/aws-k8s-tester/commit/8d4490b47d089064cf27306a59acaffaed53ab58).
  - Set via `"version-upgrade"` within `MNG` configuration.
  - e.g. `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS=..."version-upgrade":{"enable":true,"initial-wait-string":"10m","version":"1.17"}...` when `AWS_K8S_TESTER_EKS_PARAMETERS_VERSION=1.16`.
- Fix [empty default values for `CommandAfterCreateClusterOutputPath`](https://github.com/aws/aws-k8s-tester/commit/d382071054755be164a83a6a522a9bc8b0d1495a).

### `eks`

- Fix [`CheckHealth` to handle version upgrades](https://github.com/aws/aws-k8s-tester/commit/ef079fd2332e3c20c29cea1001aec1d1a95f1d87).
  - See also [`eks` health check fix](https://github.com/aws/aws-k8s-tester/commit/afe2096a5bd51b043e59ebad937ba5921c8ded98).
- Wait until [`kubectl top node` is ready](https://github.com/aws/aws-k8s-tester/commit/5301270bfa2253bda350407a2db03a38058d7e95).
- Merge [all logs from remote nodes in `eks/ng` and `eks/mng`](https://github.com/aws/aws-k8s-tester/commit/3abcece44beda975af7f3d62a35e46d06f609c7a).
- Implement [`eks/mng` version upgrades](https://github.com/aws/aws-k8s-tester/commit/3abcece44beda975af7f3d62a35e46d06f609c7a).
  - See [how it's triggered after add-on creation](https://github.com/aws/aws-k8s-tester/commit/234917535d8dd2c72b6054db72a49b56b179f16a).
  - Set via `"version-upgrade"` within `MNG` configuration.
- Move all [polling operations to own directory; `eks/ng/node-waiter` and `eks/mng/node-waiter`](https://github.com/aws/aws-k8s-tester/commit/3abcece44beda975af7f3d62a35e46d06f609c7a).
- Only upload [kubeconfig file if the file exists](https://github.com/aws/aws-k8s-tester/commit/3abcece44beda975af7f3d62a35e46d06f609c7a).
- Improve [s3 upload error message](https://github.com/aws/aws-k8s-tester/commit/e29f3038135a1e43e4e5d201ad538ad3973a0c6a).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.32.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.0) to [`v1.32.1`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.1).


<hr>


## [v1.3.5](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.5) (2020-06-11)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.4...v1.3.5).

### `eksconfig`

- Clean up [`"config-maps-"` string to `"configmaps-"`](https://github.com/aws/aws-k8s-tester/commit/6f424f20135285f24b078addbd7e3497b4e2cdf9).
  - `ADD_ON_CONFIG_MAPS_*` is now `ADD_ON_CONFIGMAPS_*`.
  - `AddOnConfigMaps*` is now `AddOnConfigmaps*`.
- Set default [`eks/app-mesh` namespace suffix as `-appmesh`, not `appmesh-system`](https://github.com/aws/aws-k8s-tester/commit/1ac87bcadc03b58bc7ea3d5f3ce79f7e03202435).

### `eks`

- Fix typo for [`kubectl top node` in `eks/kubernetes-dashboard`](https://github.com/aws/aws-k8s-tester/commit/c0892a8353c9cf8add50b0c0fda84de7c883b963).
- Set timeouts for [fargate profile delete in `eks/fargate`](https://github.com/aws/aws-k8s-tester/commit/32fb68855a6b07ad0827bb61ac0fb43063c3aa65).
- Improve [`eks/mng` delete operation](https://github.com/aws/aws-k8s-tester/commit/9878594877f32c8b2c4023a5ad0d1534b46ddda2).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.31.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.15) to [`v1.32.0`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.0).


<hr>


## [v1.3.4](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.4) (2020-06-11)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.3...v1.3.4).

### `ec2config`

- Add [`LogColorOverride`](https://github.com/aws/aws-k8s-tester/commit/5f19f50611d29a847e5f7d9b2c81affee906e564).

### `eksconfig`

- Add [`LogColorOverride`](https://github.com/aws/aws-k8s-tester/commit/5f19f50611d29a847e5f7d9b2c81affee906e564).

### `eks`

- Rename [`eks/config-maps` to `eks/configmaps`](https://github.com/aws/aws-k8s-tester/commit/d05e12ec679763eba164029435d3c8d1534baca1).
- Run [`kubectl top` when `metrics-server` is installed via `eks/kubernetes-dashboard`](https://github.com/aws/aws-k8s-tester/commit/de2049b9586fddcc2d7b94eb54b8cc48be461818).
- Fix [`panic: runtime error: invalid memory address or nil pointer dereference` in `eks/cluster.CheckHealth` panic](https://github.com/aws/aws-k8s-tester/commit/c84490b19bd845267a6263f551f79eca54d48eda).


<hr>


## [v1.3.3](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.3) (2020-06-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.2...v1.3.3).

### `eks`

- Do [not run `aws eks update-kubeconfig"` if EKS cluster creation fails](https://github.com/aws/aws-k8s-tester/commit/94cc6fb279103c93a9f1d5d8a0b4e0282a58ee52).
- Clean up [color outputs](https://github.com/aws/aws-k8s-tester/commit/4038bd07c897c3dff3107e82af360b46e9eec3a1).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.31.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.14) to [`v1.31.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.15).


<hr>


## [v1.3.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.2) (2020-06-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.1...v1.3.2).

### `eksconfig`

- Use [`kubectl` `1.17` by default](https://github.com/aws/aws-k8s-tester/pull/95).
- Add [`AddOnClusterVersionUpgrade`](https://github.com/aws/aws-k8s-tester/commit/8471fa5951d0b3f295141aba55340ef51e7fa796).
  - `AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_ENABLE`.

### `eks`

- Add [`eks/cluster`](https://github.com/aws/aws-k8s-tester/commit/8e26589bf770a261b03c4117c949ca741e04d53e).
- Add [`eks/cluster/version-upgrade`](https://github.com/aws/aws-k8s-tester/commit/8e26589bf770a261b03c4117c949ca741e04d53e).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.31.13`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.13) to [`v1.31.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.14).
- Upgrade [`github.com/kubernetes-sigs/aws-alb-ingress-controller`](https://github.com/kubernetes-sigs/aws-alb-ingress-controller/releases) from [`v1.1.7`](https://github.com/kubernetes-sigs/aws-alb-ingress-controller/releases/tag/v1.1.7) to [`v1.1.8`](https://github.com/kubernetes-sigs/aws-alb-ingress-controller/releases/tag/v1.1.8).


<hr>


## [v1.3.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.1) (2020-06-08)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.0...v1.3.1).

### `eks/mng`

- Fix [managed node group create/delete status polling](https://github.com/aws/aws-k8s-tester/commit/7cfe06785990e4f6ce14b89496c337f02c0a3f7a).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.31.10`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.10) to [`v1.31.13`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.13).
- Upgrade [`helm.sh/helm/v3`](https://github.com/helm/helm/releases) from [`v3.2.1`](https://github.com/helm/helm/releases/tag/v3.2.1) to [`v3.2.3`](https://github.com/helm/helm/releases/tag/v3.2.3).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.3.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.3.0) (2020-06-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.2.9...v1.3.0).

### `eksconfig`

- Add [`AddOnCUDAVectorAdd`](https://github.com/aws/aws-k8s-tester/pull/89).
  - Can be enabled via `AWS_K8S_TESTER_EKS_ADD_ON_CUDA_VECTOR_ADD_ENABLE`.

### `eks`

- Add [`eks/cuda-vector-add`](https://github.com/aws/aws-k8s-tester/pull/89).
- Improve [`eks/cuda-vector-add` output checks](https://github.com/aws/aws-k8s-tester/commit/75ca40a81845eba3a3b2246fb7a67f0dcc82bf8b).

### Dependency

- Upgrade [`e2e/tester/pkg` `kops` dependency to `1.17.6`](https://github.com/aws/aws-k8s-tester/pull/88).
- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.31.9`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.9) to [`v1.31.10`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.10).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

