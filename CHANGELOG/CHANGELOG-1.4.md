

<hr>



## [v1.4.8](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.8) (2020-07-20)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.7...v1.4.8).

### `cmd`

- Add [`cmd/ecr-utils`](https://github.com/aws/aws-k8s-tester/commit/50aa049a3e933c992fcfaeec6c671ec047a910bd).

### `eksconfig`

- Make [`AddOnPHPApache` ECR image configurable](https://github.com/aws/aws-k8s-tester/commit/f879a495fa1a0b7bbce8a07b25835a4edb436018).
- Add [`AddOnCNIVPC`](https://github.com/aws/aws-k8s-tester/commit/3f38808140784a06635f86a729edd0885c610136).
  - https://github.com/aws/amazon-vpc-cni-k8s/tree/master/config.
  - Requires [at least `AddOnCNIVPC.Version` `v1.7`](https://github.com/aws/aws-k8s-tester/commit/52f89b7bc78e84f6a87a081dbea277c07f54b0e5).

### `eks`

- Add [`eks/cni-vpc`](https://github.com/aws/aws-k8s-tester/commit/104f581eac40168cc3c73d04b338e995b83e0923).
  - `eks/cni-vpc` is [installed before worker nodes and not deleted](https://github.com/aws/aws-k8s-tester/commit/cb63393b3131a5266deaae2dcfcf417897bc5848).
  - Requires [at least `AddOnCNIVPC.Version` `v1.7`](https://github.com/aws/aws-k8s-tester/commit/52f89b7bc78e84f6a87a081dbea277c07f54b0e5).
- Fix [`eks/hollow-nodes/remote`]
  - Deploys to kubemark namespace
  - Uses a replication controller instead of deployment
  - Kubemark pods now equal corresponding hollow node names
  - Adds labels to hollow nodes for CA discovery
  - Idempotently Create/Update kubemark resources.

### `pkg`

- Change [`pkg/aws/ecr.Check` to return `ok bool`](https://github.com/aws/aws-k8s-tester/commit/e85f7f353d8bccb0462144219679d0945b065d04).
  - Set `true` if the repository exists.
- Add [`pkg/aws/ecr.Create`](https://github.com/aws/aws-k8s-tester/commit/e85f7f353d8bccb0462144219679d0945b065d04).
- Add [`pkg/aws/ecr.Delete`](https://github.com/aws/aws-k8s-tester/commit/e85f7f353d8bccb0462144219679d0945b065d04).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.7`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.7) to [`v1.33.8`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.8).

### Go

- Compile with [*Go 1.14.6*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.7](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.7) (2020-07-17)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.6...v1.4.7).

### `eks`

- Valid [China region service principals in `eks/ng` and `eks/mng`](https://github.com/aws/aws-k8s-tester/pull/132).
- Fix [`eks/prometheus-grafana`](https://github.com/aws/aws-k8s-tester/issues/131).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.6`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.6) to [`v1.33.7`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.7).

### Go

- Compile with [*Go 1.14.6*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.6](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.6) (2020-07-16)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.5...v1.4.6).

### `eksconfig`

- Add [`SkipDeleteClusterAndNodes` to skip EKS cluster and nodes deletion](https://github.com/aws/aws-k8s-tester/commit/565635179860b4832b64cbe3b39fdbe1c12b1ae1).
  - Useful for testing add-ons with existing clusters.
  - If true, delete operation keeps all resources created before cluster (e.g. IAM role, VPC, CMK, etc.).
  - If true, delete operation keeps all node groups and managed node groups!
  - Set via `AWS_K8S_TESTER_EKS_SKIP_DELETE_CLUSTER_AND_NODES=true`.
  - Default `AWS_K8S_TESTER_EKS_SKIP_DELETE_CLUSTER_AND_NODES=false`.
  - `aws-k8s-tester eks create cluster --path` must be passed a valid configuration file with existing cluster information in order to use the existing cluster. Create cluster once, and delete all add-ons, and keep the cluster related fields in the YAML with all other fields removed.
  - We will have better workflow and separate abstraction for add-ons. This is a short term solution.
  - See https://github.com/aws/aws-k8s-tester/issues/123 for more.
  - To delete the cluster, `SkipDeleteClusterAndNodes` must be set to `"false"` manually.
  - Create a cluster with add-ons `AWS_K8S_TESTER_EKS_SKIP_DELETE_CLUSTER_AND_NODES=true aws-k8s-tester eks create cluster --auto-path` and delete add-ons "only" with `aws-k8s-tester eks delete cluster --path [PATH]` (make sure the YAML config file is set `skip-delete-cluster-and-nodes` to `false`), and `aws-k8s-tester eks create cluster --path [PATH]` to test more add-ons. And repeat.
- Change [`AddOnHollowNodesRemote.DeploymentReplicas` to `ReplicationControllerReplicas`](https://github.com/aws/aws-k8s-tester/pull/130).
  - `AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_DEPLOYMENT_REPLICAS` is now `AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_REMOTE_REPLICATION_CONTROLLER_REPLICAS`.

### `eks`

- Use [`ReplicationController` for `eks/hollow-nodes/remote`](https://github.com/aws/aws-k8s-tester/pull/130).
- Skip [cluster and prerequisite resource deletion if `AWS_K8S_TESTER_EKS_SKIP_DELETE_CLUSTER_AND_NODES=true`](https://github.com/aws/aws-k8s-tester/commit/edcc77e163979df6919f41fb0e5552f73467d74c).

### Dependency

- Upgrade [`github.com/kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/releases) from [`v1.18.6-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.6-rc.0) to [`v1.18.7-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.7-rc.0).
- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.6-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.18.6-rc.0) to [`v0.18.7-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.18.7-rc.0).
- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.5`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.5) to [`v1.33.6`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.6).

### Go

- Compile with [*Go 1.14.6*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.5](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.5) (2020-07-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.4...v1.4.5).

### `cmd`

- Add [`cmd/s3-utils` for IRSA tests](https://github.com/aws/aws-k8s-tester/commit/3ebee3697be06d1ad6a3a9cca3788f29be2fdd1d).
- Add [`cmd/sts-utils` for IRSA tests](https://github.com/aws/aws-k8s-tester/commit/3ebee3697be06d1ad6a3a9cca3788f29be2fdd1d).

### `ec2`

- Tag [resources with user information](https://github.com/aws/aws-k8s-tester/commit/51688be5904528f48d56f102e3b3f667b8e6a723).

### `eksconfig`

- Remove [`AddOnIRSA.RoleManagedPolicyARNs`](https://github.com/aws/aws-k8s-tester/commit/aaed4fdc885ec54eee841f2ee5ebd5527c0b4afb).
- Rename [`SSHConfig` to `NodeInfo`](https://github.com/aws/aws-k8s-tester/commit/aaed4fdc885ec54eee841f2ee5ebd5527c0b4afb).
- [`AWS_K8S_TESTER_EKS_PARAMETERS_TAGS` must be set in `map[string]string`](https://github.com/aws/aws-k8s-tester/commit/d7f79677949ee58f7cb4c37d176d5f05caa7dacf).
  - `'a=b;c;d,e=f'` should be `{"a":"b","c":"d"}`.

### `eks`

- Clean up [ELB resources from services in `eks/wordpress,nlb-guestbook,jupyter-hub,prometheus-grafana`](https://github.com/aws/aws-k8s-tester/commit/7aada081e7f3bd5486000f3afb4072bc7f8eac94).
- Tag [resources with user information](https://github.com/aws/aws-k8s-tester/commit/51688be5904528f48d56f102e3b3f667b8e6a723).
- Set [timeouts for `eks/irsa` and `eks/irsa-fargate` S3 requests](https://github.com/aws/aws-k8s-tester/commit/a8a1ef411854636946868a5a815e1e7dd089dd26).
- Allow [`eks/irsa` tester failures, only requires minimum 1 Pod success, debugging...](https://github.com/aws/aws-k8s-tester/commit/a89d1606946d363fb02fd853fc2f26d35463e0b7).
- Add [`kubectl logs --timestamps` flags](https://github.com/aws/aws-k8s-tester/commit/a89d1606946d363fb02fd853fc2f26d35463e0b7).
- Remove [`arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess` from default `eks/irsa-fargate` IAM role](https://github.com/aws/aws-k8s-tester/commit/a89d1606946d363fb02fd853fc2f26d35463e0b7).
- Fix [`eks/irsa` when worker nodes have disabled SSH access (e.g. Bottlerocket OS)](https://github.com/aws/aws-k8s-tester/commit/a89d1606946d363fb02fd853fc2f26d35463e0b7).
- Fix [typos in `kubectl describe job` commands for `eks/*/remote` testers](https://github.com/aws/aws-k8s-tester/commit/561cdfe9b1fa9137e9b6c468fcf731e886daa094).
- Fix [wrong buckets for `eks/*/remote` testers](https://github.com/aws/aws-k8s-tester/commit/01903baa70764b796c92c4da9397bca19c6bda05).
- Add [more debugging logs to remote testers for failed pods](https://github.com/aws/aws-k8s-tester/commit/a3e1e97f1d92e109c84edeba6e91d09f1e5fcd17).
- Add [more debugging logs to `eks/irsa`](https://github.com/aws/aws-k8s-tester/commit/a3e1e97f1d92e109c84edeba6e91d09f1e5fcd17).
- Fix [remote tester job Pod](https://github.com/aws/aws-k8s-tester/commit/0d6e2c9e390b688029cc88d565b249ce79f4e15c).
  - Change pod `RestartPolicy` from `v1.RestartPolicyOnFailure` to `v1.RestartPolicyNever`.
  - ref. https://github.com/kubernetes/kubernetes/issues/54870.
- Use [`pkg/k8s-client.WaitForDeploymentCompletes`](https://github.com/aws/aws-k8s-tester/commit/0d6e2c9e390b688029cc88d565b249ce79f4e15c).
- Fix [`eks/irsa` and `eks/irsa-fargate` role ARN query tests](https://github.com/aws/aws-k8s-tester/commit/12d17a25229faa3f2daf573cab3fc0c4aeaa0076).
  - Now using [`s3-utils`](https://github.com/aws/aws-k8s-tester/commit/c66af9eaf47a8d48470c8986c08681d8c20c3012).
- Use [regional STS endpoint by default](https://github.com/aws/aws-k8s-tester/commit/9a09a37d92dbd8eed2f98a9249aa6e3b2f9d6459).
- Add [`iam:SetSecurityTokenServicePreferences` to worker node role IAM policy](https://github.com/aws/aws-k8s-tester/commit/9a09a37d92dbd8eed2f98a9249aa6e3b2f9d6459).
  - ref. https://docs.aws.amazon.com/cli/latest/reference/iam/set-security-token-service-preferences.html.
  - To use regional STS endpoint.
- Fix [STS regional endpoint dial timeouts](https://github.com/aws/aws-k8s-tester/commit/bce310faa1c2abcc48617f6b5a3c732992a039d0).
  - e.g. `"RequestError: send request failed\ncaused by: Post \"https://iam.amazonaws.com/\": dial tcp: i/o timeout"} failed to create AWS session RequestError: send request failed caused by: Post "https://sts.us-west-2.amazonaws.com/": dial tcp: i/o timeout"`
- Allow [`eks/fluentd` namespace deletion timeouts](https://github.com/aws/aws-k8s-tester/commit/ff5200fecb55b842dfeb0e338e19f49906e91d3c).

### `pkg`

- Add [`pkg/user`](https://github.com/aws/aws-k8s-tester/commit/ae786d8017115860c600d9e5b52a04375372d4bd).
- Add [`pkg/aws/s3.WithTimeout`](https://github.com/aws/aws-k8s-tester/commit/8ba8a4b59b64031b654301a61b1f468f96e1d260).
- Fix [`pkg/fileutil.IsDirWriteable` `os.RemoveAll`](https://github.com/aws/aws-k8s-tester/commit/c251476f3efc313d91f8d93401613ffbfb6fbd9c).
  - Fix `"failed to write dir remove /var/log/.touch: no such file or directory"` in remote testers.
- Add [`pkg/k8s-client.WaitForDeploymentCompletes`](https://github.com/aws/aws-k8s-tester/commit/a8a69c5e092abf88ff7e0ddb636c4ce8400cf2f1).
- Add [`pkg/k8s-client.WithPodFunc` to debug job pod failures](https://github.com/aws/aws-k8s-tester/commit/f245f770980daacf9f462a0d62c3c95c845a1477).
- Use [regional STS endpoint by default in `pkg/aws`](https://github.com/aws/aws-k8s-tester/commit/6f1a3f830933713e17f0c059532d3cd77fa2587e).


```diff
# Upgrading Kubernetes to "v1.19" is reverted...
-### Dependency
-
-- Upgrade [`github.com/kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/releases) from [`v1.18.6-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.6-rc.0) to [`v1.19.0-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.19.0-rc.0).
-- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.6-rc.0`](https://github.com/kubernetes/clienthttps://github.com/kubernetes/client-go/releases/tag/v0.18.6-rc.0) to [`v0.19.0-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.19.0-rc.0).
-  - See [commit `0e4cbc8e` for all the `eks` changes](https://github.com/aws/aws-k8s-tester/commit/0e4cbc8e0a3b7c7f3808e40205ecf5dc6d3ddbe9).
-  - See [commit `f1a984e3` for all the `vendor` changes](https://github.com/aws/aws-k8s-tester/commit/f1a984e394c880a1864327f97bb54ffab94e48f8).
-  - ref. https://github.com/kubernetes/kubernetes/pull/90552 changes `k8s.io/kubernetes/pkg/kubelet/remote` to `k8s.io/kubernetes/pkg/kubelet/cri/remote`.
```

```bash
# github.com/containerd/containerd/sys
vendor/github.com/containerd/containerd/sys/proc.go:33:34: undefined: system.GetClockTicks
github.com/google/cadvisor/container/raw
# github.com/google/cadvisor/container/raw
vendor/github.com/google/cadvisor/container/raw/handler.go:62:20: undefined: "github.com/opencontainers/runc/libcontainer/cgroups/fs".Manager
# github.com/google/cadvisor/container/docker
vendor/github.com/google/cadvisor/container/docker/handler.go:140:20: undefined: "github.com/opencontainers/runc/libcontainer/cgroups/fs".Manager
# github.com/google/cadvisor/container/containerd
vendor/github.com/google/cadvisor/container/containerd/handler.go:72:20: undefined: "github.com/opencontainers/runc/libcontainer/cgroups/fs".Manager
github.com/google/cadvisor/container/crio
# github.com/google/cadvisor/container/crio
vendor/github.com/google/cadvisor/container/crio/handler.go:74:23: undefined: "github.com/opencontainers/runc/libcontainer/cgroups/fs".Manager
vendor/github.com/google/cadvisor/container/crio/handler.go:98:20: undefined: "github.com/opencontainers/runc/libcontainer/cgroups/fs".Manager
github.com/aws/aws-k8s-tester/pkg/aws
github.com/aws/aws-k8s-tester/eks/cluster-loader
github.com/aws/aws-k8s-tester/pkg/k8s-client
# helm.sh/helm/v3/pkg/kube
vendor/helm.sh/helm/v3/pkg/kube/client.go:180:26: too many arguments in call to helper.Get
vendor/helm.sh/helm/v3/pkg/kube/client.go:180:58: info.Export undefined (type *"k8s.io/cli-runtime/pkg/resource".Info has no field or method Export)
vendor/helm.sh/helm/v3/pkg/kube/client.go:380:31: too many arguments in call to helper.Get
vendor/helm.sh/helm/v3/pkg/kube/client.go:380:69: target.Export undefined (type *"k8s.io/cli-runtime/pkg/resource".Info has no field or method Export)
vendor/helm.sh/helm/v3/pkg/kube/client.go:485:11: undefined: "k8s.io/client-go/tools/watch".ListWatchUntil
```

### Dependency

- Upgrade [`helm.sh/helm/v3`](https://github.com/helm/helm/releases) from [`v3.2.3`](https://github.com/helm/helm/releases/tag/v3.2.3) to [`v3.2.4`](https://github.com/helm/helm/releases/tag/v3.2.4).
  - [`v3.3.0-rc.1`](https://github.com/helm/helm/releases/tag/v3.3.0-rc.1) does not work...
  - ref. `kubectl -n grafana logs pod/grafana-test` shows `[ "$code" == "200" ]' failed`.

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).




<hr>



## [v1.4.4](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.4) (2020-07-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.3...v1.4.4).

### `eksconfig`

- Make [EKS 1.17 default version](https://github.com/aws/aws-k8s-tester/commit/d924eb6b082e2fe678717057b138ce1ac964f2d9).
  - https://github.com/aws/containers-roadmap/issues/697

### `eks`

- Fix [node label for BottleRocket OS worker nodes](https://github.com/aws/aws-k8s-tester/commit/86e8266052b0cc5aafd303168d764ec9fa8f5771).
- Increase [`ListCSRs` batch limit to 1,000](https://github.com/aws/aws-k8s-tester/commit/4965374b15ec09224477f41cc4b1c024601dfb43).
- Increase [`ListNodes` batch limit to 1,000](https://github.com/aws/aws-k8s-tester/commit/7d36a80c22cfddbed20f75600462a6a396277d8a).
- Do [not print spinner if not supported](https://github.com/aws/aws-k8s-tester/commit/afcac86d06e66b74488232f9d2c6d883b7c7832f).
- Set [upper limit for `WaitForJobCompletes`](https://github.com/aws/aws-k8s-tester/commit/7d36a80c22cfddbed20f75600462a6a396277d8a).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).




<hr>




## [v1.4.3](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.3) (2020-07-10)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.2...v1.4.3).

### `ec2`

- Only [print `SSMCommands` when total nodes are less than 10](https://github.com/aws/aws-k8s-tester/commit/464f1bf0a903f9c1b26c0e408ba1bc274ed94bb4).

### `eksconfig`

- Fix [`AddOnConformance` `sonobuoy-result/plugins/e2e/results/global/e2e.log` upload path](https://github.com/aws/aws-k8s-tester/commit/718c7dd1533541a23e99f45403a17d06af3ef1b7).

### `eks`

- Clean up [worker nodes polling outputs](https://github.com/aws/aws-k8s-tester/commit/e6788aff97b996327653d82dd7fb8f15e7e10cf3).
- Only [print `SSMCommands` when total nodes are less than 10](https://github.com/aws/aws-k8s-tester/commit/464f1bf0a903f9c1b26c0e408ba1bc274ed94bb4).
- Fix [`eks/conformance` `sonobuoy-result/plugins/e2e/results/global/e2e.log` upload](https://github.com/aws/aws-k8s-tester/commit/718c7dd1533541a23e99f45403a17d06af3ef1b7).
- Add ["spinner" to all polling functions](https://github.com/aws/aws-k8s-tester/commit/0f8f81969c238c16e59f83e2d4bb6e5f85bdbeac).

### `pkg`

- Add [`pkg/spinner`](https://github.com/aws/aws-k8s-tester/commit/2d0aa8a696d85914f1081a92a5a40f7f5d6ffbe9).
- Pass [log writer to `pkg/cfn.Poll` for "spinner"](https://github.com/aws/aws-k8s-tester/commit/2d0aa8a696d85914f1081a92a5a40f7f5d6ffbe9).
- Increase [list pod batch limit and reduce batch interval for `pkg/k8s-client.WaitForJobCompletes`](https://github.com/aws/aws-k8s-tester/commit/81866ec90463636f970fbee680b703df6fcb15fd).
- Retry [`pkg/k8s-client.ListPod` when a paginated response is stale](https://github.com/aws/aws-k8s-tester/commit/3097ecf6a6cfb65fee021de883dbad612114c839).
  - Fix `"The provided continue parameter is too old to display a consistent list result. You can start a new list without the continue parameter, or use the continue token in this response to retrieve the remainder of the results. Continuing with the provided token results in an inconsistent list - objects that were created, modified, or deleted between the time the first chunk was returned and now may show up in the list."`.

### Dependency

- Add [`github.com/briandowns/spinner`](https://github.com/briandowns/spinner/releases) [`v1.11.1`](https://github.com/briandowns/spinner/releases/tag/v1.11.1).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.2) (2020-07-09)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.1...v1.4.2).

### `ec2`

- Handle [duplicate EC2 key pair creation error](https://github.com/aws/aws-k8s-tester/commit/3dcaaa7c799d2e5fd5b7d42f18533a36fae37178).

### `eks`

- Fix [`eks/ng` ASG launch template for multiple instance types](https://github.com/aws/aws-k8s-tester/commit/5cb687b5df9240668a331ac14193b2bcedee74f0).
- Handle [duplicate EC2 key pair creation error](https://github.com/aws/aws-k8s-tester/commit/3dcaaa7c799d2e5fd5b7d42f18533a36fae37178).
- Wait [`AddOnClusterVersionUpgrade.WaitBeforeUpgrade` before running cluster version upgrades](https://github.com/aws/aws-k8s-tester/commit/a254e0df3e10e59adee30851a692d104d173ec9f).
- Fix [instance state polling in `eks/ng` and `eks/mng`](https://github.com/aws/aws-k8s-tester/commit/7ee2e8c2887d2d61e596cff793c591490f681ac3).

### `eksconfig`

- Add [`AddOnClusterVersionUpgrade.WaitBeforeUpgrade`](https://github.com/aws/aws-k8s-tester/commit/a254e0df3e10e59adee30851a692d104d173ec9f).
  - Set via `AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_VERSION_UPGRADE_WAIT_BEFORE_UPGRADE=5m`.

### `pkg`

- Pass [`context.Context` and `chan struct{}` to `pkg/aws/ec2.PollUntilRunning`](https://github.com/aws/aws-k8s-tester/commit/4916d16c5e9c68f7fae5d11be2ba6df43898a280).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.33.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.3) to [`v1.33.5`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.5).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.1) (2020-07-07)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.4.0...v1.4.1).

### `ec2`

- Fix [log fetch, make it work run multiple times](https://github.com/aws/aws-k8s-tester/commit/fab79e552cc89a749f45e5f5e001b6faaea467ee).

### `eks`

- Add [`eks/cw-agent` and `eks/fluentd`](https://github.com/aws/aws-k8s-tester/pull/119).
- Add [`eks/php-apache`](https://github.com/aws/aws-k8s-tester/pull/119).
- Support [ECR repository for `busybox` images to minimize docker hub dependency in `eks/cron-jobs` and `eks/jobs-echo`](https://github.com/aws/aws-k8s-tester/pull/118).
- Handle [`NotFound` errors in delete operations](https://github.com/aws/aws-k8s-tester/commit/2d7b30d58b1fb6b3d90635d9e32824615d972c28).
- Fix [log fetch, make it work run multiple times](https://github.com/aws/aws-k8s-tester/commit/fab79e552cc89a749f45e5f5e001b6faaea467ee).
- Increase [`MNG` update timeouts](https://github.com/aws/aws-k8s-tester/commit/43f826bda28b276aa0cae5d289a71fc3fc77a148).
- Create [regional ECR client to all remote testers based on `RepositoryRegion`](https://github.com/aws/aws-k8s-tester/commit/4de5e9763d06475da5ee1e61e935218f32fafb85).

### `eksconfig`

- Set [`AddOnNodeGroups.FetchLogs` to `false` by default](https://github.com/aws/aws-k8s-tester/pull/122), to reduce the test runtime for a large number of worker nodes.
  - Set `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_FETCH_LOGS=true` to enable.
  - To stream pod logs to CloudWatch, set `AWS_K8S_TESTER_EKS_ADD_ON_FLUENTD_ENABLE=true`.
- Set [`AddOnManagedNodeGroups.FetchLogs` to `false` by default](https://github.com/aws/aws-k8s-tester/pull/122), to reduce the test runtime for a large number of worker nodes.
  - Set `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_FETCH_LOGS=true` to enable.
  - To stream pod logs to CloudWatch, set `AWS_K8S_TESTER_EKS_ADD_ON_FLUENTD_ENABLE=true`.
- Add [`AddOnCWAgent` and `AddOnFluentd`](https://github.com/aws/aws-k8s-tester/pull/119).
- Add [`AddOnPHPApache`](https://github.com/aws/aws-k8s-tester/pull/119).
- Support [ECR repository for `busybox` images to minimize docker hub dependency in `AddOnCronJobs` and `AddOnJobsEcho`](https://github.com/aws/aws-k8s-tester/pull/118).
- Add [`RepositoryRegion` to all remote testers](https://github.com/aws/aws-k8s-tester/commit/4de5e9763d06475da5ee1e61e935218f32fafb85).
- Reduce [`AddOnIRSA` default replicas to 1](https://github.com/aws/aws-k8s-tester/pull/112).

### `pkg/aws/ecr`

- Add [region checks to `Check`](https://github.com/aws/aws-k8s-tester/commit/4de5e9763d06475da5ee1e61e935218f32fafb85).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.32.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.11) to [`v1.33.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.33.3).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).



<hr>



## [v1.4.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.0) (2020-06-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.9...v1.4.0).

### `ec2-utils`

- [`ec2-utils --auto-path` now automatically uses the generated cluster name for local file paths, instead of random string](https://github.com/aws/aws-k8s-tester/commit/53b51d38b1aa4e6ea1454cc631c9511dcbe4267a).

### `ec2config`

- Enable [`S3BucketCreate` and `S3BucketCreateKeep` by default, error if no S3 bucket is specified](https://github.com/aws/aws-k8s-tester/commit/7d743b2d3cedb55079c080457ab662c09f6fcd03).

### `aws-k8s-tester`

- [`aws-k8s-tester --auto-path` now automatically uses the generated cluster name for local file paths, instead of random string](https://github.com/aws/aws-k8s-tester/commit/53b51d38b1aa4e6ea1454cc631c9511dcbe4267a).
- Remove [`--block` flags](https://github.com/aws/aws-k8s-tester/commit/cdf83863700a4fb52a38484b56fedeb7c6b1eb78).
- Print [JSON body for (managed) node groups](https://github.com/aws/aws-k8s-tester/commit/7810477b7ece0e1609625d53bf56eabaaa9df145).

### `ec2`

- Clean up [S3 uploads](https://github.com/aws/aws-k8s-tester/commit/d2cd3b516c667f556641216218047ea522b70945).
- Clean up [`colorstring` printf](https://github.com/aws/aws-k8s-tester/pull/101).

### `eks`

- Set [timeouts for `"aws sts get-caller-identity"` for `eks/irsa` and `eks/irsa-fargate`](https://github.com/aws/aws-k8s-tester/commit/562dbc864778fee918c1c5ba2ed8d893b2e2c09c).
- Improve and clean up [all `Poll` functions with contexts](https://github.com/aws/aws-k8s-tester/commit/96ee2b8a9223ad147e06b53b6f627c0886e06094).
- Add [missing `"configmaps"` to `eks/stresser/remote` RBAC](https://github.com/aws/aws-k8s-tester/commit/7df3fbd3fa815214cf8e01a8722e9ee0e1907456).
- Clean up [`eks/irsa-fargate`](https://github.com/aws/aws-k8s-tester/commit/8a022ff8a9adf949084a21aaf3821ab80d133613).
- Use [multi-writer to pipe stderr logging to log file](https://github.com/aws/aws-k8s-tester/commit/2c2c9e9993e19eb24093570078a1f502febc371b).
- Run [query function while checking `eks/mng` version upgrades](https://github.com/aws/aws-k8s-tester/commit/4019d6d25990430551f7235e5fc2afebe6f34047).
- Improve and clean up [`eks/irsa`](https://github.com/aws/aws-k8s-tester/commit/12bf8c74cab92df3877606347cf5748ff8d3b89b).
- Add [`clusterloader --provider=eks` flag to `eks/cluster-loader`](https://github.com/aws/aws-k8s-tester/commit/dc406f03528902a318dabac10e824c3c06e2dd06).
- Add [`eks/cluster-loader` `CL2UseHostNetworkPods` support](https://github.com/aws/aws-k8s-tester/commit/23310e17d172491c44158a7d07290e2d172e5fdc).
- Explicitly [set `RestartPolicy` in all pod objects](https://github.com/aws/aws-k8s-tester/commit/5f133714d33eae57196237f88f17538fc2a4cdde).
- Run [`eks/mng/scale.Scale` after creating add-ons](https://github.com/aws/aws-k8s-tester/commit/dc43773768e58a54ffda2f7d755ab345ceed8a2a).
- Fix [`eks/mng/scale`](https://github.com/aws/aws-k8s-tester/commit/44014bfb896ccce7344ee414bc14b4dca77c4491).
- Update [nodes after `eks/mng/scale`](https://github.com/aws/aws-k8s-tester/commit/6fd1e3c533e5e319302fa8170ddda3d45ae04c2d).
- Remove [`eks/tester.Tester.AggregateResults`](https://github.com/aws/aws-k8s-tester/commit/be028bd4d8430347788adb98636fb7b78da132fe).
- `eks/cluster-loader` with [`Job` object to run remote testers "once", instead of `Deployment`](https://github.com/aws/aws-k8s-tester/commit/252358916c22b7840d688916d52f62e06810e744).
- `eks/cluster-loader` to remove [log fetcher dependency for remote tester, use S3 instead](https://github.com/aws/aws-k8s-tester/commit/252358916c22b7840d688916d52f62e06810e744).
- `eks/configmaps` with [`Job` object to run remote testers "once", instead of `Deployment`](https://github.com/aws/aws-k8s-tester/commit/a7519fb3b7251c8f60dcf248fea0801be59e5a08).
- `eks/configmaps` to remove [log fetcher dependency for remote tester, use S3 instead](https://github.com/aws/aws-k8s-tester/commit/a7519fb3b7251c8f60dcf248fea0801be59e5a08).
- `eks/csrs` with [`Job` object to run remote testers "once", instead of `Deployment`](https://github.com/aws/aws-k8s-tester/commit/ad17054ae05b01d287a60e34ef413d0ba5864529).
- `eks/csrs` to remove [log fetcher dependency for remote tester, use S3 instead](https://github.com/aws/aws-k8s-tester/commit/ad17054ae05b01d287a60e34ef413d0ba5864529).
- `eks/secrets` with [`Job` object to run remote testers "once", instead of `Deployment`](https://github.com/aws/aws-k8s-tester/commit/2aa633af7744b7feeb30bc877bee96409b7715b7).
- `eks/secrets` to remove [log fetcher dependency for remote tester, use S3 instead](https://github.com/aws/aws-k8s-tester/commit/2aa633af7744b7feeb30bc877bee96409b7715b7).
- `eks/stresser` with [`Job` object to run remote testers "once", instead of `Deployment`](https://github.com/aws/aws-k8s-tester/commit/cc3246a38d1dd7019fe41901fa50bf9e1662077e).
- `eks/stresser` to remove [log fetcher dependency for remote tester, use S3 instead](https://github.com/aws/aws-k8s-tester/commit/cc3246a38d1dd7019fe41901fa50bf9e1662077e).
- Clean up [S3 uploads](https://github.com/aws/aws-k8s-tester/commit/d2cd3b516c667f556641216218047ea522b70945).
- Compare [raw data points for regression tests](https://github.com/aws/aws-k8s-tester/commit/021dc585cc59114fe0a9343c47c111f7f1a25b98).
  - Used for [Kolmogorov–Smirnov test](https://en.wikipedia.org/wiki/Kolmogorov%E2%80%93Smirnov_test).
- Publish [performance data to CloudWatch](https://github.com/aws/aws-k8s-tester/commit/038fd83e6a180d5a98287b508d243661b23a356a).
- Add [compare tests for all stressing tests, useful for regression tests](https://github.com/aws/aws-k8s-tester/commit/28939738fd0ca8aeaf512fef43f706472650ab13).
- Improve [`eks/configmaps`, `eks/csrs`, `eks/secrets` results collect with S3](https://github.com/aws/aws-k8s-tester/commit/a8500fbf1b9218ca587d265daed6a6845b3ebfcb).
- Add [`eks/tester.Tester.Name` method](https://github.com/aws/aws-k8s-tester/commit/2f8f08595d53f18abe77c47a6f43c6e734127f53).
- Fix [`eks/stresser` collect metrics](https://github.com/aws/aws-k8s-tester/commit/2f8f08595d53f18abe77c47a6f43c6e734127f53).
- Clean up [`colorstring` printf](https://github.com/aws/aws-k8s-tester/pull/101).
- Clean up [polling operation error handling](https://github.com/aws/aws-k8s-tester/commit/26627f14f4dbbcc8dd64d6307ed6e58c0b809f52).
  - Rename [`eks/cluster/version-upgrade.Poll` to `eks/cluster/wait.PollUpdate`](https://github.com/aws/aws-k8s-tester/commit/a6eeea26a7ab3c7069a4278026b56de87707c9b1).
- Discard [HTTP download progress for URL checks](https://github.com/aws/aws-k8s-tester/commit/d54e2c7b125d22779b014fb0eb0ac72e165b2350).
- Increase [`cluster-loader` timeout and error if output is not expected](https://github.com/aws/aws-k8s-tester/commit/13ff01fad653249435770138069ef600b0c873fa).
- Run [`AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER` before `AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES`](https://github.com/aws/aws-k8s-tester/commit/c0377d39377c47da86677028209585b854046e1a).
  - *TODO*: Handle `23 resource_gather_worker.go:63] error while reading data from hollowdreamprimehaze8iw5ul: the server is currently unable to handle the request (get nodes hollowdreamprimehaze8iw5ul:10250)`.
- Improve [`eks/cluster-loader/remote` result download, use `"scp"`](https://github.com/aws/aws-k8s-tester/commit/a3c9d7d5e3382c378de686fe0faec6bdeb47f027).
- Store [`kube-apiserver` `/metrics` output](https://github.com/aws/aws-k8s-tester/commit/9e7985fe8ffc948866e792d0984faafbf4e57c59).
- Add [`eks/cluster-loader.ParsePodStartupLatency`](https://github.com/aws/aws-k8s-tester/commit/322cd88e94e879157f6b409f9c604fdbbc95e465).
- Add [`eks/cluster-loader.MergePodStartupLatency`](https://github.com/aws/aws-k8s-tester/commit/322cd88e94e879157f6b409f9c604fdbbc95e465).
- Record [`PodStartupLatency` in `eksconfig.AddOnClusterLoaderLocal` via `eks/cluster-loader/local`](https://github.com/aws/aws-k8s-tester/commit/8d4cb87b7bd798ad7f1b5d2de22d0deb26c4c75e).
- Record [`PodStartupLatency` in `eksconfig.AddOnClusterLoaderRemote` via `eks/cluster-loader/remote`](https://github.com/aws/aws-k8s-tester/commit/8d4cb87b7bd798ad7f1b5d2de22d0deb26c4c75e).
- Fix [`eks/cluster-loader` error handling](https://github.com/aws/aws-k8s-tester/commit/4a9d982929c32efbdfad820e0cece67721d53034).
- Add [S3 access policies to worker node roles](https://github.com/aws/aws-k8s-tester/commit/bcf0b1da501fc9a1bcf1a7691e690e729ee95b59).
- Improve [`eks/stresser/remote` results fetch](https://github.com/aws/aws-k8s-tester/commit/a982c1b484d8133b113bfa1f22df6698411898b7).
- Fix [multiple `eks/cluster-loader` runs](https://github.com/aws/aws-k8s-tester/commit/a7d6ebc79d76782d5bbff533183d9baa05bd663e).
- Add [extra namespace force-deletion function to `eks/stresser/remote`](https://github.com/aws/aws-k8s-tester/commit/dc6ef6849a57d2236bc23a0a89413a7b377a211c).
- [`eks/mng/scale` added to scale mngs up and down](https://github.com/aws/aws-k8s-tester/pull/106)
  - See https://docs.aws.amazon.com/cli/latest/reference/eks/update-nodegroup-config.html.

### `eksconfig`

- `AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_DEPLOYMENT_REPLICAS` is now [`AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_COMPLETES` and `AWS_K8S_TESTER_EKS_ADD_ON_CSRS_REMOTE_PARALLELS`](https://github.com/aws/aws-k8s-tester/commit/e322c9d80b7c1280a399d1b69ae2dbda4b7ee23e).
- `AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_DEPLOYMENT_REPLICAS` is now [`AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_COMPLETES` and `AWS_K8S_TESTER_EKS_ADD_ON_CONFIGMAPS_REMOTE_PARALLELS`](https://github.com/aws/aws-k8s-tester/commit/e322c9d80b7c1280a399d1b69ae2dbda4b7ee23e).
- `AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_DEPLOYMENT_REPLICAS` is now [`AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_COMPLETES` and `AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_REMOTE_PARALLELS`](https://github.com/aws/aws-k8s-tester/commit/e322c9d80b7c1280a399d1b69ae2dbda4b7ee23e).
- `AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_DEPLOYMENT_REPLICAS` is now [`AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_COMPLETES` and `AWS_K8S_TESTER_EKS_ADD_ON_STRESSER_REMOTE_PARALLELS`](https://github.com/aws/aws-k8s-tester/commit/e322c9d80b7c1280a399d1b69ae2dbda4b7ee23e).
- Use [`Job` for all remote testers](https://github.com/aws/aws-k8s-tester/commit/d8bec1505b6f4d3b1e70b7129278629bff14e321).
- Enable [`S3BucketCreate` and `S3BucketCreateKeep` by default, error if no S3 bucket is specified](https://github.com/aws/aws-k8s-tester/commit/7d743b2d3cedb55079c080457ab662c09f6fcd03).
- Configure [S3 directory](https://github.com/aws/aws-k8s-tester/commit/53a0169e208b66a00135bf05002c27de2000e9ed).
 - Add [`ClusterAutoscaler` add-on per node-group using `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS={"GetRef.Name-...":{..."cluster-autoscaler":{"enable":false}...}}`](https://github.com/aws/aws-k8s-tester/pull/99).
- Fix [typo in `eksconfig.AddOnManagedNodeGroups.LogsTarGzPath`](https://github.com/aws/aws-k8s-tester/commit/7b60047ca4d6fad281db512d4de905a27b80303a).
- Add [`Status.PrivateDNSToNodeInfo` for node SSH access](https://github.com/aws/aws-k8s-tester/commit/a3c9d7d5e3382c378de686fe0faec6bdeb47f027).
- Record [`PodStartupLatency` in `eksconfig.AddOnClusterLoaderLocal` via `eks/cluster-loader/local`](https://github.com/aws/aws-k8s-tester/commit/8d4cb87b7bd798ad7f1b5d2de22d0deb26c4c75e).
- Record [`PodStartupLatency` in `eksconfig.AddOnClusterLoaderRemote` via `eks/cluster-loader/remote`](https://github.com/aws/aws-k8s-tester/commit/8d4cb87b7bd798ad7f1b5d2de22d0deb26c4c75e).
- Add [`RequestsSummaryWritesCompareS3Dir` and `RequestsSummaryReadsCompareS3Dir`](https://github.com/aws/aws-k8s-tester/commit/e559fae84787e7936fd167cd7da9a893c691e856).
- Add [`AddOnClusterLoader*` `CL2UseHostNetworkPods` support](https://github.com/aws/aws-k8s-tester/commit/23310e17d172491c44158a7d07290e2d172e5fdc).

### `ssh`

- Move [`"scp"` executable binary path check before creating context timeouts](https://github.com/aws/aws-k8s-tester/commit/4c3950e6582745684a9d628c5c0ea355e3f7edc1).
- Fix and [improve retries](https://github.com/aws/aws-k8s-tester/commit/949cc1ea63131ce7d27808d7fc12d6e988d07978).

### `pkg`

- Add [`pkg/k8s-client.WaitForJobCompletes,WaitForCronJobCompletes`](https://github.com/aws/aws-k8s-tester/commit/98ecccc5f9ca3c1a2b0ba2713abae089bb169794).
- Add [`pkg/aws/s3`](https://github.com/aws/aws-k8s-tester/commit/a982c1b484d8133b113bfa1f22df6698411898b7).
- Add [`pkg/k8s-client.EKSConfig.MetricsRawOutputDir` to store `kube-apiserver` `/metrics` output](https://github.com/aws/aws-k8s-tester/commit/9e7985fe8ffc948866e792d0984faafbf4e57c59).
- Add [`pkg/k8s-client.WithForceDelete` option for `DeleteNamespaceAndWait`](https://github.com/aws/aws-k8s-tester/commit/803ba2d263227adea026fcf1bb5262ebb2abd230).
  - Fix https://github.com/aws/aws-k8s-tester/issues/100.
  - See [`kubernetes/kubernetes#60807`](https://github.com/kubernetes/kubernetes/issues/60807).
- Add [`pkg/metrics.RequestsCompare`](https://github.com/aws/aws-k8s-tester/commit/00b7c5c922f77db2243fb8d5c26c0e0f9fd9d484).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.32.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.2) to [`v1.32.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.11).
- Upgrade [`github.com/kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/releases) from [`v1.18.2`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.2) to [`v1.18.6-rc.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.18.6-rc.0).
- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.3`](https://github.com/kubernetes/clienthttps://github.com/kubernetes/client-go/releases/tag/v0.18.3) to [`v0.18.6-rc.0`](https://github.com/kubernetes/client-go/releases/tag/v0.18.6-rc.0).
  - See [commit `fc93a579` for all the changes](https://github.com/aws/aws-k8s-tester/commit/fc93a5792c7334fc099e18ad4a4de394f8c2a35c).
- Add [`k8s.io/perf-tests`](https://github.com/kubernetes/perf-tests/releases).
  - See [`1aea23d3` for commit](https://github.com/aws/aws-k8s-tester/commit/1aea23d3259794307b45d344d3a953238c394efb).

### Go

- Compile with [*Go 1.14.4*](https://golang.org/doc/devel/release.html#go1.14).



<hr>


