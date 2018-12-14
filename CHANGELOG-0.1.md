

<hr>


## [0.1.5](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.5) (2018-12-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.4...0.1.5).

### `eksconfig`

- Require [Kubernetes v1.11](https://github.com/aws/aws-k8s-tester/commit/62cbfe2ba32d1faf752cb336b9665aa6726bc286).


<hr>


## [0.1.4](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.4) (2018-12-03)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.3...0.1.4).

### `internal`

- [mkdir before downloading binaries](https://github.com/aws/aws-k8s-tester/commit/0b4e0177ea8669cafee870b19c105d80c9549cd5).


<hr>


## [0.1.3](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.3) (2018-11-29)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/0.1.2...0.1.3).

### `aws-k8s-tester` CLI

- Add [`aws-k8s-tester kubeadm`](https://github.com/aws/aws-k8s-tester/commit/7339ba312212eff2afed720be0e8b0484f50c7bc) command.

### `ec2config`

- Add [`"install-start-kubeadm-amazon-linux-2"` plugin](https://github.com/aws/aws-k8s-tester/commit/fe378db6e272ce37430d07dfddfa84d7c0d1199b).
- Rename to [`ec2config.Config.IngressCIDRs` to `ec2config.Config.IngressRulesTCP`](https://github.com/aws/aws-k8s-tester/commit/a5b1b1479c895e59496bd240990dcc9bfd7924a1).
- Rename to [`"install-start-docker-amazon-linux-2"` and `"install-start-docker-ubuntu"`](https://github.com/aws/aws-k8s-tester/commit/fe378db6e272ce37430d07dfddfa84d7c0d1199b).
- Rename to [`"install-go1.11.2"` to `"install-go-1.11.2"`](https://github.com/aws/aws-k8s-tester/commit/2a9ae7cd967dbd9e67c899de81c05f64ae634db9)
- Remove [`"install-docker-ubuntu"`](https://github.com/aws/aws-k8s-tester/commit/fe378db6e272ce37430d07dfddfa84d7c0d1199b).
- Remove [`"install-start-kubeadm-ubuntu"`](https://github.com/aws/aws-k8s-tester/commit/fe378db6e272ce37430d07dfddfa84d7c0d1199b).
- Set [`ec2config.Config.Wait` to true by default](https://github.com/aws/aws-k8s-tester/commit/fe378db6e272ce37430d07dfddfa84d7c0d1199b).
- Add [`ec2config.Config.InstanceProfileName`](https://github.com/aws/aws-k8s-tester/pull/15/commits/2f7f19c775faea8b6244b8e579716b6bea297f3c)
- Fix a typo in [`ec2config.Config.SubnetIDToAvailabilityZone`](https://github.com/aws/aws-k8s-tester/pull/15/commits/06269a8c296fd28aacd80588982b787f62c0a14d) field.

### `eksconfig`

- Disable [log uploads by default](https://github.com/aws/aws-k8s-tester/commit/cebd3bb6ac5a0d94076c53eb25d8597631fc5c43).
- Add [`eksconfig.Config.KubectlDownloadURL`](https://github.com/aws/aws-k8s-tester/commit/3b8704fcf0c15229fc3480caca41b4ddec1497a1) field.
- Add [`eksconfig.Config.KubectlPath`](https://github.com/aws/aws-k8s-tester/commit/c5826538bac54764c5368f86b85ab46fcf4c54a5) field.
- Add [`eksconfig.Config.AWSIAMAuthenticatorDownloadURL`](https://github.com/aws/aws-k8s-tester/commit/3b8704fcf0c15229fc3480caca41b4ddec1497a1) field.
- Add [`eksconfig.Config.AWSIAMAuthenticatorPath`](https://github.com/aws/aws-k8s-tester/commit/c5826538bac54764c5368f86b85ab46fcf4c54a5) field.
- Fix a typo in [`eksconfig.Config.WorkerNodeASG*`](https://github.com/aws/aws-k8s-tester/commit/e2ed8f45da472660f20743701b266fa79b5611d8) field.
- [Add new regions, update AMIs](https://github.com/aws/aws-k8s-tester/commit/017b53add758cb6ad8e74eda69bb09bc80c76faa).

### `ekstester`

- Add [`ekstester.Tester.KubectlCommand`](https://github.com/aws/aws-k8s-tester/commit/8608df45d6e6cb07c06cd84504a1ae52fb08a1f6) method.

### `storagetester`

- Rename to [`storagetester` from `etcdtester`](https://github.com/aws/aws-k8s-tester/commit/81f38f66690f6f0616b809c4fe8e1860d78b4346).

### `kubeadmconfig`

- Add [`kubeadmconfig`](https://github.com/aws/aws-k8s-tester/commit/857de963f493202b1b89d4d7c26e01c7cc304da0).

### `internal`

- Improve [worker node log fetcher](https://github.com/aws/aws-k8s-tester/pull/10) with concurrency.
- Add [retries on `kubectl get all` fail](https://github.com/aws/aws-k8s-tester/pull/8).
- Add [`kubectl` and `aws-iam-authenticator` downloader](https://github.com/aws/aws-k8s-tester/commit/3b8704fcf0c15229fc3480caca41b4ddec1497a1).
- Handle [interrupt and terminate signals when creating a cluster](https://github.com/aws/aws-k8s-tester/pull/14).
- Add [`csi` pkg and move most of csi integration testing to here](https://github.com/aws/aws-k8s-tester/pull/13).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.15.65`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.65) to [`v1.15.87`](https://github.com/aws/aws-sdk-go/releases/tag/v1.15.87).
- Change [`github.com/ghodss/yaml`](https://github.com/ghodss/yaml/releases) to [`sigs.k8s.io/yaml`](https://github.com/kubernetes-sigs/yaml).

### Go

- Compile with [*Go 1.11.2*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.1.2](https://github.com/aws/aws-k8s-tester/releases/tag/0.1.2) (2018-11-05)

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
- Use [Amazon Linux 2 for `aws-k8s-tester csi test integration`](https://github.com/aws/aws-k8s-tester/commit/88a90939d1fc4f798e3ff2a35c10b2aa1b562c14) command.

### `eksconfig`

- Add [`eksconfig.ALBIngressController.TestScalabilityMinutes`](https://github.com/aws/aws-k8s-tester/commit/10240a423f62e991bf4ef0f051f7a24d9340daf6gqq) field.
- Remove [`eksconfig.Instance` field and replace it with `ec2config.Instance](https://github.com/aws/aws-k8s-tester/commit/5156d0df502fe43a89b9c45fcfd3cecb96856d74)

### `etcdconfig`

- Add [`etcdconfig`](https://github.com/aws/aws-k8s-tester/pull/7) package for etcd conformance tests.

### `etcdtester`

- Add [`etcdtester`](https://github.com/aws/aws-k8s-tester/pull/7) package for etcd conformance tests.
  - Use [bastion instance to run test operations](https://github.com/aws/aws-k8s-tester/commit/8e7fd780a16433adce69c54c1d995a53a34d60e9).
  - To be moved to upstream etcd test project.

### `internal`

- Update [`Tag` field to suffix S3 bucket name with user ID and hostname](https://github.com/aws/aws-k8s-tester/commit/7bfdd6417bcb7128cc00ab1e7810a106bac94347).
- Rename to [`ec2config` from `internal/ec2/config`](https://github.com/aws/aws-k8s-tester/commit/f8b5d466966862658dff6bc254d7491ba2333aa6).
- Add [`ec2config.Config.IngressCIDRs`](https://github.com/aws/aws-k8s-tester/commit/8e7fd780a16433adce69c54c1d995a53a34d60e9) field.
- Add [`ec2config.Config.Wait`](https://github.com/aws/aws-k8s-tester/commit/6073c2de289e352c5454d4b17380022168bcbac6) field.
- Add [`internal/ssh.SSH.Send/Download` using SCP protocol](https://github.com/aws/aws-k8s-tester/commit/84e4363ad658cc6db8e0bf979f6f6bb841795eec).
- Implement [`internal/ec2.Deployer.Delete`](https://github.com/aws/aws-k8s-tester/commit/000d2292d6108e1ea46ce359f6ac9a08214b592f) method.

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
- Add [`ec2config.Config.InitScriptCreated`](https://github.com/aws/aws-k8s-tester/commit/793935db2418a7c960d89512372f534996adcb19) flag.
- Add [`ec2config.Instance.LaunchTime`](https://github.com/aws/aws-k8s-tester/commit/36fe5579ffb719d108272640c22f478127295dac) field.

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

