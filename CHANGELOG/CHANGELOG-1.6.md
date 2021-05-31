

<hr>

## [v1.5.9](https://github.com/aws/aws-k8s-tester/releases/tag/v1.5.9) (2021)

### `client`

- Rename [`KubeConfig*` to `Kubeconfig*`](https://github.com/aws/aws-k8s-tester/commit/e7e10253a44a33ff9c16955a39df43d9e412c944).

### `eksconfig`

- Set [default EKS version to 1.20](https://github.com/aws/aws-k8s-tester/commit/8f6b05193721b19378cdd1c69f6f2d787341d1f2).

### `k8s-tester`

- Rename [`EnablePrompt` to `Prompt`](https://github.com/aws/aws-k8s-tester/commit/e7e10253a44a33ff9c16955a39df43d9e412c944).
- Improve [ELB deletion in `k8s-tester/nlb-hello-world`](https://github.com/aws/aws-k8s-tester/commit/288c27cb9922164743cc9e7af5c2443e238147d5).
- Add [`k8s-tester/jobs-echo`](https://github.com/aws/aws-k8s-tester/commit/7d05190c873f3166fcf55f75832b40cc74826944).
- Add [`k8s-tester/jobs-pi`](https://github.com/aws/aws-k8s-tester/commit/5a188f1874876ad4228c02afdb99da730418763a).
- Add [`k8s-tester/metrics-server`](https://github.com/aws/aws-k8s-tester/commit/b95ed4f88e8143c5b94a5e66448718bf513abf9b).
- Add [`k8s-tester/kubernetes-dashboard`](https://github.com/aws/aws-k8s-tester/commit/ebe96e838950abc14f1016532e715112d5624f01).
- Add [`k8s-tester/cloudwatch-agent`](https://github.com/aws/aws-k8s-tester/commit/e46ea545846a662e0e950ee70facfec6e060b5de).
- Add [`k8s-tester/helm`](https://github.com/aws/aws-k8s-tester/commit/2a2c739f085bec0b4d8d7b2bae0789abe4d54c65).
- Add [`k8s-tester/csi-ebs`](https://github.com/aws/aws-k8s-tester/commit/075fe2234e9fa0bc14a4b2a314db70ab45670e1a).
- Add [`k8s-tester/php-apache`](https://github.com/aws/aws-k8s-tester/commit/a9a70d681e491f9f22ffcad025cc2601ee47cde1).
- Add [`k8s-tester/configmaps`](https://github.com/aws/aws-k8s-tester/commit/TODO).
- Add [`k8s-tester/conformance`](https://github.com/aws/aws-k8s-tester/commit/TODO).

### Dependency

- Upgrade [`go.uber.org/zap`](https://github.com/uber-go/zap/releases) from [`v1.16.0`](https://github.com/uber-go/zap/releases/tag/v1.16.0) to [`v1.17.0`](https://github.com/uber-go/zap/releases/tag/v1.17.0).

### Go

- Compile with [*Go 1.16.4*](https://golang.org/doc/devel/release.html#go1.16).


<hr>
