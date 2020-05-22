

<hr>


## [v1.2.3](https://github.com/aws/aws-k8s-tester/releases/tag/v1.2.3) (2020-05-21)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.2.2...v1.2.3).

### `eks`

- `eks/nlb-hello-world` uses [`AddOnNLBHelloWorld.DeploymentNodeSelector` to overwrite node selector](https://github.com/aws/aws-k8s-tester/commit/107e88f1c7b4f6d859eb386138cae5e792c71a86).
- `eks/alb-2048` uses [`AddOnALB2048.DeploymentNodeSelector2048` to overwrite node selector](https://github.com/aws/aws-k8s-tester/commit/107e88f1c7b4f6d859eb386138cae5e792c71a86).

### `eksconfig`

- Add [optional `AddOnNLBHelloWorld.DeploymentNodeSelector`](https://github.com/aws/aws-k8s-tester/commit/107e88f1c7b4f6d859eb386138cae5e792c71a86).
  - e.g. `AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_DEPLOYMENT_NODE_SELECTOR='{"a":"b","c":"d"}'`.
- Add [optional `AddOnALB2048.DeploymentNodeSelector2048`](https://github.com/aws/aws-k8s-tester/commit/107e88f1c7b4f6d859eb386138cae5e792c71a86).
  - e.g. `AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_DEPLOYMENT_NODE_SELECTOR_2048='{"a":"b","c":"d"}'`.

### Dependency

- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`v0.18.2`](https://github.com/kubernetes/clienthttps://github.com/kubernetes/client-go/releases/tag/v0.18.2) to [`v0.18.3`](https://github.com/kubernetes/client-go/releases/tag/v0.18.3).
- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.29`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.29) to [`v1.31.3`](https://github.com/aws/aws-sdk-go/releases/tag/v1.31.3).
- Upgrade [`github.com/prometheus/client_golang`](https://github.com/prometheus/client_golang/releases) from [`v1.0.0`](https://github.com/prometheus/client_golang/releases/tag/v1.0.0) to [`v1.6.0`](https://github.com/prometheus/client_golang/releases/tag/v1.6.0).
- Upgrade [`go.etcd.io/etcd`](https://github.com/etcd-io/etcd/releases) from [`59f5fb25a533`](https://github.com/etcd-io/etcd/commit/59f5fb25a5333adb32377f517ea81daf66992713) to [`54ba9589114f` (`v3.4.9`)](https://github.com/etcd-io/etcd/commit/54ba9589114fc3fa5cc36c313550b3c0c0938c91).

### Go

- Compile with [*Go 1.14.3*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.2.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.2.2) (2020-05-17)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.2.1...v1.2.2).

### `aws-k8s-tester`

- Rename [`aws-k8s-tester eks create cluster-loader` to `aws-k8s-tester eks create stresser`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`aws-k8s-tester eks create config-maps`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`aws-k8s-tester eks create csrs`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`aws-k8s-tester eks create secrets`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).

### `eksconfig`

- Rename [`AddOnSecrets` to `AddOnSecretsLocal` and `AddOnSecretsRemote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - `ADD_ON_SECRETS_*` is now `ADD_ON_SECRETS_LOCAL_` (or `ADD_ON_SECRETS_REMOTE_`).
- Rename [`AddOnConfigMaps` to `AddOnConfigMapsLocal` and `AddOnConfigMapsRemote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - `ADD_ON_CONFIG_MAPS_*` is now `ADD_ON_CONFIG_MAPS_LOCAL_` (or `ADD_ON_CONFIG_MAPS_REMOTE_`).
- Rename [`AddOnCSRs` to `AddOnCSRsLocal` and `AddOnCSRsRemote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - `ADD_ON_CSRS_*` is now `ADD_ON_CSRS_LOCAL_` (or `ADD_ON_CSRS_REMOTE_`).
- Rename [`AddOnClusterLoader*` to `AddOnStresser*`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - Add [`AddOnStresserLocal.RequestsSummaryWrite*`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - Add [`AddOnStresserRemote.RequestsSummaryWrite*`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - `ADD_ON_CLUSTER_LOADER_*` is now `ADD_ON_STRESSER_*`.

### `eks`

- Update [`eks/alb-2048` to use `aws-alb-ingress-controller` `v1.1.7` with new `wafv2:*` IAM permissions](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
  - See https://github.com/kubernetes-sigs/aws-alb-ingress-controller/releases/tag/v1.1.7.
  - Add [`wafv2:*` to node groups](https://github.com/aws/aws-k8s-tester/commit/a0c7d18428538ef7f69eaf3d0e5af4c9887d8f98).
  - e.g. `controller.go:217] kubebuilder/controller "msg"="Reconciler error" "error"="failed get WAFv2 webACL for load balancer arn:aws:elasticloadbalancing:us-west-2:607362164682:loadbalancer/app/7fbd7e3d-eks2020051714char-ad37/26de385bd4f0a46a: AccessDeniedException: User: arn:aws:sts::607362164682:assumed-role/eks-2020051714-charisma6fxe-role-ng/i-06acdfc5db3ccf8fd is not authorized to perform: wafv2:GetWebACLForResource on resource: arn:aws:wafv2:us-west-2:607362164682:regional/webacl/*\n\tstatus code: 400, request id: 3c6b7245-b68a-43e5-af74-92e994670229"  "controller"="alb-ingress-controller" "request"={"Namespace":"eks-2020051714-charisma6fxe-alb-2048","Name":"alb-2048-ingress"}`
- Rename [`eks/alb` to `eks/alb-2048`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Rename [`eks/nlb` to `eks/nlb-hello-world`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Rename [`eks/cluster-loader` to `eks/cluster-stresser`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Rename directory [`eks/configmaps` to `eks/config-maps`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`eks/config-maps/local` and `eks/config-maps/remote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Rename directory [`eks/cronjobs` to `eks/cron-jobs`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`eks/cron-jobs/local` and `eks/cron-jobs/remote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`eks/secrets/local` and `eks/secrets/remote`](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).
- Add [`NodeType=regular` node labels (to differentiate from hollow nodes, `NodeType=hollow-nodes`)](https://github.com/aws/aws-k8s-tester/commit/84a53b6c73f51dd9babb98f0b2eb04ad8d7618fe).

### Dependency

- Upgrade [`helm.sh/helm/v3`](https://github.com/helm/helm/releases) from [`v3.2.0`](https://github.com/helm/helm/releases/tag/v3.2.0) to [`v3.2.1`](https://github.com/helm/helm/releases/tag/v3.2.1).

### Go

- Compile with [*Go 1.14.3*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.2.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.2.1) (2020-05-15)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.2.0...v1.2.1).

### `eks`

- Fix [`ecriface.ECRAPI.DescribeRepositories` calls](https://github.com/aws/aws-k8s-tester/commit/cc418cc3e8c01727c94c3b8fa8099775106020f5).

### `eksconfig`

- Add [`RepositoryAccountID` fields to `AddOnFargate`, `AddOnIRSA`, `AddOnIRSAFargate`, `AddOnHollowNodesRemote`, `AddOnClusterLoaderRemote`](https://github.com/aws/aws-k8s-tester/commit/cc418cc3e8c01727c94c3b8fa8099775106020f5).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.28`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.28) to [`v1.30.29`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.29).

### Go

- Compile with [*Go 1.14.3*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.2.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.2.0) (2020-05-15)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.1.9...v1.2.0).

### `ec2config`

- Improve [README](https://github.com/aws/aws-k8s-tester/commit/4a15ae1d61cf58d286263c16e6074f8e3745077a).

### `eksconfig`

- Improve [README](https://github.com/aws/aws-k8s-tester/commit/4a15ae1d61cf58d286263c16e6074f8e3745077a).
- Remove [unnecessary fields from `AddOnIRSA`](https://github.com/aws/aws-k8s-tester/commit/52666165f7564922deb2e6e304c1f1c73412d691).
- Remove [unnecessary fields from `AddOnIRSAFargate`](https://github.com/aws/aws-k8s-tester/commit/52666165f7564922deb2e6e304c1f1c73412d691).
- Now [`AddOnFargate` optionally takes remote ECR image](https://github.com/aws/aws-k8s-tester/commit/afc73f3a7e77d817b953c5e4fe76be82f30fb6ff).
  - `AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_NAME` is optional.
  - `AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_URI` is optional.
  - `AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_REPOSITORY_IMAGE_TAG` is optional.
  - See https://github.com/aws/aws-k8s-tester/blob/master/Dockerfile and https://github.com/aws/aws-k8s-tester/blob/master/Makefile for container image build instructions.
- Now [`AddOnIRSA` requires remote ECR image](https://github.com/aws/aws-k8s-tester/commit/afc73f3a7e77d817b953c5e4fe76be82f30fb6ff).
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_NAME` is now required.
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_URI` is now required.
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_REPOSITORY_IMAGE_TAG` is now required.
  - See https://github.com/aws/aws-k8s-tester/blob/master/Dockerfile and https://github.com/aws/aws-k8s-tester/blob/master/Makefile for container image build instructions.
- Now [`AddOnIRSAFargate` requires remote ECR image](https://github.com/aws/aws-k8s-tester/commit/afc73f3a7e77d817b953c5e4fe76be82f30fb6ff).
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_NAME` is now required.
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_URI` is now required.
  - `AWS_K8S_TESTER_EKS_ADD_ON_IRSA_FARGATE_REPOSITORY_IMAGE_TAG` is now required.
  - See https://github.com/aws/aws-k8s-tester/blob/master/Dockerfile and https://github.com/aws/aws-k8s-tester/blob/master/Makefile for container image build instructions.

### `eks`

- Fix [`eks/hollow-nodes` with RBAC and clean up node labels](https://github.com/aws/aws-k8s-tester/commit/0f2c940680a8b1d430bf5726d6217d246cfa1ca2).
  - Previously, it did not work because of `"NodeRestriction"` from `"kube-apiserver --enable-admission-plugins"`. Now it works with `"NodeRestriction"`.
  - Add [`nodes/status` resource group](https://github.com/aws/aws-k8s-tester/commit/0aff1fb25565fc94d8fcadfe84c1f97c9ad6325d).
  - Add [`pods/status` resource group](https://github.com/aws/aws-k8s-tester/commit/0aff1fb25565fc94d8fcadfe84c1f97c9ad6325d).
- Fix and improve [`eks/irsa` configmap tests](https://github.com/aws/aws-k8s-tester/commit/52666165f7564922deb2e6e304c1f1c73412d691).
- Fix and improve [`eks/irsa-fargate` configmap tests](https://github.com/aws/aws-k8s-tester/commit/52666165f7564922deb2e6e304c1f1c73412d691).
- Improve [`eks/cluster-loader` `RequestSummary` output and separate results for reads](https://github.com/aws/aws-k8s-tester/commit/968fa2a18001112ca6c952439fe0a45b0dfd2b85).

### `pkg/aws/ssm`

- Check [`ssm.ListCommandInvocationsInput` batch limit](https://github.com/aws/aws-k8s-tester/commit/23d21857342930ceb0e165628ba8c124fb99198d).

### `pkg/metrics`

- Add [`HistogramBuckets.Table` method](https://github.com/aws/aws-k8s-tester/commit/52666165f7564922deb2e6e304c1f1c73412d691).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.26`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.26) to [`v1.30.28`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.28).

### Go

- Compile with [*Go 1.14.3*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

