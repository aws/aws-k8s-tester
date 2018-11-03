

<hr>


## [0.1.2](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.2) (2018-10-30)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.1...0.1.2).

### Project

- Rename to [`aws-k8s-tester`](https://github.com/aws/aws-k8s-tester/commit/1512e69443705eafe0ad5b4440e325d2f374cf73).
- Rename to [`ekstester` from `eksdeployer`](https://github.com/aws/aws-k8s-tester/commit/e56f2bd4554ebe26421c896d6b3ae2993d19e6ac).
- Add [`bill-of-materials.json`](https://github.com/aws/aws-k8s-tester/pull/7).

### `aws-k8s-tester` CLI

- Rename to [`aws-k8s-tester` from `awstester`](https://github.com/aws/aws-k8s-tester/commit/1512e69443705eafe0ad5b4440e325d2f374cf73).
- Remove [`aws-k8s-tester ec2 wait`](https://github.com/aws/aws-k8s-tester/commit/36a74c699819d92abdf7f89028ea95b54f19fc98) command.
- Add [`aws-k8s-tester wrk --run-in-ec2`](https://github.com/aws/aws-k8s-tester/commit/3f62032c0fe5aecda5f69a64fe528d46807cb5a5) flag.
- Change [`aws-k8s-tester wrk --duration` to `aws-k8s-tester wrk --minutes`](https://github.com/aws/aws-k8s-tester/commit/8c04dd324ae1e8c915779af4f8c0e8f5b3ca3ecc) flag.
- Add [`aws-k8s-tester etcd`](https://github.com/aws/aws-k8s-tester/commit/3af0d30bc9b85ca800122ff732502d9f820249bb) command.

### `eksconfig`

- Add [`eksconfig.ALBIngressController.TestScalabilityMinutes`](https://github.com/aws/aws-k8s-tester/commit/10240a423f62e991bf4ef0f051f7a24d9340daf6gqq) field.
- Remove [`eksconfig.Instance` field and replace it with `ec2config.Instance](https://github.com/aws/aws-k8s-tester/commit/5156d0df502fe43a89b9c45fcfd3cecb96856d74)

### `etcdconfig`

- Add [`etcdconfig`](https://github.com/aws/aws-k8s-tester/pull/7) package for etcd conformance tests.

### `etcdtester`

- Add [`etcdtester`](https://github.com/aws/aws-k8s-tester/pull/7) package for etcd conformance tests.
  - To be moved to upstream etcd test project.

### `internal`

- Rename to [`ec2config` from `internal/ec2/config`](https://github.com/aws/aws-k8s-tester/commit/f8b5d466966862658dff6bc254d7491ba2333aa6).
- Add [`internal/ec2/config.Config.IngressTCPPorts`](https://github.com/aws/aws-k8s-tester/commit/f0f9dbf808a7eb26d63b089d0ba5e4a58b8bccc0) field.
- Add [`internal/ec2/config.Config.Wait`](https://github.com/aws/aws-k8s-tester/commit/6073c2de289e352c5454d4b17380022168bcbac6) field.
- Add [`internal/ssh.SSH.Send/Download` using SCP protocol](https://github.com/aws/aws-k8s-tester/commit/84e4363ad658cc6db8e0bf979f6f6bb841795eec).

### `pkg/wrk`

- Change [`wrk.Config.Duration` to `wrk.Config.Minutes`](https://github.com/aws/aws-k8s-tester/commit/133f7945e297a01c367d021b924c7a04ff992a9e) flag.

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.64`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.64) to [`v1.15.65`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.65).

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.1.1](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.1) (2018-10-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.0...0.1.1).

### `awstester` CLI

- Add [`awstester ec2 wait`](https://github.com/aws/aws-k8s-tester/commit/8f66f7413f8f32a8479888ba3ae53449e75d05cc) command.
- Use EC2 metadata to name [`awstester wrk` output](https://github.com/aws/aws-k8s-tester/commit/03ec0af6e12d4ca85e539905b7ec3da2729c1f3f).
- Split [`awstester eks prow status-serve/get` to `awstester eks prow status serve` and `aws-k8s-tester eks prow status get`](https://github.com/aws/aws-k8s-tester/commit/297bf2795c4bc62c55de121b47e0a1bb62ad6108).

### `eksconfig`

- Add [`eksconfig.Instance.LaunchTime`](https://github.com/aws/aws-k8s-tester/commit/d886cbeb0d7ea9b8e71f0b9bf57e04923985202d) field.

### `internal`

- Add [`"install-kubeadm"` plugin to `internal/ec2/config/plugins`](https://github.com/aws/aws-k8s-tester/commit/e103c1ca68742bb56a8c43d3508d0c09423bb6b5).
- Add [`internal/ec2/config.Config.InitScriptCreated`](https://github.com/aws/aws-k8s-tester/commit/793935db2418a7c960d89512372f534996adcb19) flag.
- Add [`internal/ec2/config.Instance.LaunchTime`](https://github.com/aws/aws-k8s-tester/commit/36fe5579ffb719d108272640c22f478127295dac) field.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.1.0](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.0) (2018-10-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.0.9...0.1.0).

### `awstester` CLI

- Add [`awstester eks prow status-get --data-dir`](https://github.com/aws/aws-k8s-tester/commit/034b9f6667b664368bace942b2e8f160c1eadf9f) flag.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

