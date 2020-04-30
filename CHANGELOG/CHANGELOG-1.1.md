

<hr>


## [v1.1.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.1.2) (2020-04-30)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.1.1...v1.1.2).

### `eksconfig`

- Replace [`AddOnConfigMaps.QPS`, `AddOnCSRs.QPS`, `AddOnSecrets.SecretsQPS`, `AddOnSecrets.PodQPS` with `ClientQPS`](https://github.com/aws/aws-k8s-tester/commit/35a3f0f5b7356d0c5e5871e30e5c25df3d6e18c3).
- Replace [`AddOnConfigMaps.Burst`, `AddOnCSRs.Burst`, `AddOnSecrets.SecretsBurst`, `AddOnSecrets.PodBurst` with `ClientBurst`](https://github.com/aws/aws-k8s-tester/commit/35a3f0f5b7356d0c5e5871e30e5c25df3d6e18c3).
- Make [`1.16` as default EKS version](https://github.com/aws/aws-k8s-tester/commit/3ec335f6e3478ee67f8bbc98a6628ce0ed26a5e4).
- Add [`AddOnCSRs.FailThreshold`, `AddOnConfigMaps.FailThreshold` and `AddOnSecrets.FailThreshold`](https://github.com/aws/aws-k8s-tester/commit/1ca247353e98152fe8ef21ba14e06ad85e0d22a8).

### `eks`

- Add [`listJobs` and `listCronJobs` to cluster loader](https://github.com/aws/aws-k8s-tester/commit/a4eb3fe1367275fdc8c413efe3c0751e81907e4c).
- Fix [`eks/csrs` parallel creation](https://github.com/aws/aws-k8s-tester/commit/8fd9a10a8a1f2c604fd19fe969ca9a858c15d60b).
- Clean up [`eks/csi-ebs` command outputs](https://github.com/aws/aws-k8s-tester/commit/a72d437aad67021fc34e3ee091d8bdb281effad8).
- Fix [`eks/job` query](https://github.com/aws/aws-k8s-tester/commit/11f8d9faf768983b713cf46a8be51accbfddcaca).
- Use [`AddOnCSRs.FailThreshold`, `AddOnConfigMaps.FailThreshold` and `AddOnSecrets.FailThreshold`](https://github.com/aws/aws-k8s-tester/commit/1ca247353e98152fe8ef21ba14e06ad85e0d22a8).
- Open [`1-10000` node group ports for upstream conformance tests (e.g. guestbook)](https://github.com/aws/aws-k8s-tester/commit/1e45a83b000d9a314ff1181ac0acf5bd3f3d98ca).
- Update [results output to `UP SUCCESS/FAIL` and `DOWN SUCCESS/FAIL`](https://github.com/aws/aws-k8s-tester/commit/5e7937ea974df455b8e07786a0b2f7aed3e7d6a8).
- Reorder [add-on deployment order to create `AddOnKubernetesDashboard` and `AddOnPrometheusGrafana` first](https://github.com/aws/aws-k8s-tester/commit/5234fb7a1515a44aa947cf48ddcb7062fdce6c45).
  - Otherwise, `prometheus-server` Pod was being evicted due to `"The node was low on resource: memory. Container prometheus-server was using 3885112Ki, which exceeds its request of 0. Container prometheus-server-configmap-reload was using 2100Ki, which exceeds its request of 0."`.
- Fix [`eks/irsa` test results count](https://github.com/aws/aws-k8s-tester/commit/1b702d5d4519438105c5de231597bf558ab8112e).
- Improve [`kubectl logs/exec` outputs in `eks/fargate` and `eks/irsa-fargate`](https://github.com/aws/aws-k8s-tester/commit/9418e475d056a2cf791f2e3a4f8aa87cc41c4ec6).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.16`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.16) to [`v1.30.18`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.18).


<hr>


## [v1.1.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.1.1) (2020-04-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.1.0...v1.1.1).

### `eksconfig`

- Reorder [`AddOnCSIEBS`](https://github.com/aws/aws-k8s-tester/commit/a6429ad9566427f6515ad6579f5ae619b31d19af).

### `eks`

- Fix [`nvidia-smi` Pod tests](https://github.com/aws/aws-k8s-tester/commit/ccaf87bbd6c3dc281f33e9fd52d058406bd7cb12).
- Fix [`eks/gpu` `InstallNvidiaDriver`](https://github.com/aws/aws-k8s-tester/commit/be9a0febf05d4e361a26069ae6accea4f8fdeaf2).
- Fix [`eks/csi-ebs` `ebs-plugin` log fetch](https://github.com/aws/aws-k8s-tester/commit/ccaf87bbd6c3dc281f33e9fd52d058406bd7cb12).
- Improve [`eks/helm` `QueryFunc` output](https://github.com/aws/aws-k8s-tester/commit/ccaf87bbd6c3dc281f33e9fd52d058406bd7cb12).
- Improve [`eks/gpu` `nvidia-device-plugin-daemonset` logs](https://github.com/aws/aws-k8s-tester/commit/a66de07db067e6e2ee56749c522f841f65fa6c64).
- Add [health check after load testing](https://github.com/aws/aws-k8s-tester/commit/f6bea5e350a665dff4f628720adc8e564e2b6670).
- Clean up [`eks/gpu` `kubectl` outputs](https://github.com/aws/aws-k8s-tester/commit/0d99ab95c6ae3e645a5dffd8e8934f33c1592437).
- Improve [`eks/csi-ebs` `app=ebs-csi-node` debugging outputs](https://github.com/aws/aws-k8s-tester/commit/aac285d62a6570007ee502a37e784575ff81fb5f).
- Improve [`eks/helm` error message](https://github.com/aws/aws-k8s-tester/commit/89e7039ab99ea1377fc88fa0de38190533c21d74).
- Add [`eks/helm.InstallConfig.LogFunc`](https://github.com/aws/aws-k8s-tester/commit/86c2867ac0e0f56010dba27b9bb64cb87ba4eed7).
- Upload [artifacts to S3 after cluster creation](https://github.com/aws/aws-k8s-tester/commit/912da1f877424871df5b4f21e6217da6d619bae1).
- Reorder [`eks/csi-ebs` installation](https://github.com/aws/aws-k8s-tester/commit/a6429ad9566427f6515ad6579f5ae619b31d19af).
- Add [more load testing cases to `eks/cluster-loader`](https://github.com/aws/aws-k8s-tester/commit/e7f33ed67339dcc6abbf29e98bf22946f0fe1c05).

### `pkg/k8s-client`

- Fetch [`ServerVersionInfo` in health check](https://github.com/aws/aws-k8s-tester/commit/9b099c6e31ffe0e64c1d6c7ef9dafa31ebf13bcf).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.15) to [`v1.30.16`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.16).


<hr>


## [v1.1.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.1.0) (2020-04-28)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.9...v1.1.0).

### `ec2`

- Fix [VPC creation template for 2-AZ regions](https://github.com/aws/aws-k8s-tester/commit/c8f4e888d4249cc4934be335672d096b37479eec).

### `eks`

- Fix [VPC creation template for 2-AZ regions](https://github.com/aws/aws-k8s-tester/commit/c8f4e888d4249cc4934be335672d096b37479eec).
- Logs [`CSI` `EBS` daemon-set driver logs](https://github.com/aws/aws-k8s-tester/commit/a77c3c33710324e9ec8d98fa76a75ca3a68cba89).
- Add [`List` endpoints and secrets to `eks/cluster-loader`](https://github.com/aws/aws-k8s-tester/commit/a3d69d50a5298f54b4b9e516dcc3578d7b35cecb).

### `eksconfig`

- Add [`Config.CommandAfterCreateClusterTimeout` and `Config.CommandAfterCreateAddOnsTimeout`](https://github.com/aws/aws-k8s-tester/commit/558cccb8cf01554c365784509815c88470ec58c9).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.14`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.14) to [`v1.30.15`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.15).


<hr>

