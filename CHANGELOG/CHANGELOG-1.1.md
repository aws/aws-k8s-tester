

<hr>


## [v1.1.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.1.1) (2020-04-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.1.0...v1.1.1).

### `eksconfig`

- Reorder [`AddOnCSIEBS`](https://github.com/aws/aws-k8s-tester/commit/).

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
- Reorder [`eks/csi-ebs` installation](https://github.com/aws/aws-k8s-tester/commit/).

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

