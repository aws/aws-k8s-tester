

<hr>


## [v1.4.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.4.1) (2020-07)

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
- Add [`Status.PrivateDNSToSSHConfig` for node SSH access](https://github.com/aws/aws-k8s-tester/commit/a3c9d7d5e3382c378de686fe0faec6bdeb47f027).
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


<hr>

