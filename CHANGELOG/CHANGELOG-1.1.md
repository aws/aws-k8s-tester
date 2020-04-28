

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

