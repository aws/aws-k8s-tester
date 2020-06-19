

<hr>


## [v1.4.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.0) (2020-06)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.3.9...v1.4.0).

### `ec2-utils`

- [`ec2-utils --auto-path` now automatically uses the generated cluster name for local file paths, instead of random string](https://github.com/aws/aws-k8s-tester/commit/53b51d38b1aa4e6ea1454cc631c9511dcbe4267a).

### `aws-k8s-tester`

- [`aws-k8s-tester --auto-path` now automatically uses the generated cluster name for local file paths, instead of random string](https://github.com/aws/aws-k8s-tester/commit/53b51d38b1aa4e6ea1454cc631c9511dcbe4267a).

### `ec2`

- Clean up [`colorstring` printf](https://github.com/aws/aws-k8s-tester/pull/101).

### `eks`

- Clean up [`colorstring` printf](https://github.com/aws/aws-k8s-tester/pull/101).
- Clean up [polling operation error handling](https://github.com/aws/aws-k8s-tester/commit/26627f14f4dbbcc8dd64d6307ed6e58c0b809f52).
  - Rename [`eks/cluster/version-upgrade.Poll` to `eks/cluster/wait.PollUpdate`](https://github.com/aws/aws-k8s-tester/commit/a6eeea26a7ab3c7069a4278026b56de87707c9b1).
- Discard [HTTP download progress for URL checks](https://github.com/aws/aws-k8s-tester/commit/d54e2c7b125d22779b014fb0eb0ac72e165b2350).
- Increase [`cluster-loader` timeout and error if output is not expected](https://github.com/aws/aws-k8s-tester/commit/13ff01fad653249435770138069ef600b0c873fa).
- Run [`AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER` before `AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES`](https://github.com/aws/aws-k8s-tester/commit/c0377d39377c47da86677028209585b854046e1a).
  - *TODO*: Handle `23 resource_gather_worker.go:63] error while reading data from hollowdreamprimehaze8iw5ul: the server is currently unable to handle the request (get nodes hollowdreamprimehaze8iw5ul:10250)`.

### `eksconfig`

 - [Add ClusterAutoScaler add-on per node-group using `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS={"GetRef.Name-...":{..."cluster-autoscaler":{"enable":false}...}}`](https://github.com/aws/aws-k8s-tester/pull/99).
- Fix [typo in `eksconfig.AddOnManagedNodeGroups.LogsTarGzPath`](https://github.com/aws/aws-k8s-tester/commit/7b60047ca4d6fad281db512d4de905a27b80303a).

### `ssh`

- Move [`"scp"` executable binary path check before creating context timeouts](https://github.com/aws/aws-k8s-tester/commit/4c3950e6582745684a9d628c5c0ea355e3f7edc1).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.32.2`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.2) to [`v1.32.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.32.3).
- Upgrade [`github.com/kubernetes/kubernetes`](https://github.com/kubernetes/kubernetes/releases) from [`v0.18.2`](https://github.com/kubernetes/clienthttps://github.com/kubernetes/kubernetes/releases/tag/v0.18.3) to [`v0.18.4`](https://github.com/kubernetes/kubernetes/releases/tag/v0.18.4).
- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.3`](https://github.com/kubernetes/clienthttps://github.com/kubernetes/client-go/releases/tag/v0.18.3) to [`v0.18.4`](https://github.com/kubernetes/client-go/releases/tag/v0.18.4).
  - See [commit `fc93a579` for all the changes](https://github.com/aws/aws-k8s-tester/commit/fc93a5792c7334fc099e18ad4a4de394f8c2a35c).


<hr>

