

<hr>


## [0.1.2](https://github.com/aws/awstester/releases/tag/0.1.2) (2018-10-30)

See [code changes](https://github.com/aws/awstester/compare/0.1.1...0.1.2).

### `awstester` CLI

- Add [`awstester ec2 create instances --wait`](https://github.com/aws/awstester/commit/9a62f8d69347cfe3b65b9862e2f5faf4c50f972b) flag.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.1.1](https://github.com/aws/awstester/releases/tag/0.1.1) (2018-10-29)

See [code changes](https://github.com/aws/awstester/compare/0.1.0...0.1.1).

### `awstester` CLI

- Add [`awstester ec2 wait`](https://github.com/aws/awstester/commit/8f66f7413f8f32a8479888ba3ae53449e75d05cc) command.
- Use EC2 metadata to name [`awstester wrk` output](https://github.com/aws/awstester/commit/03ec0af6e12d4ca85e539905b7ec3da2729c1f3f).
- Split [`awstester eks prow status-serve/get` to `awstester eks prow status serve` and `awstester eks prow status get`](https://github.com/aws/awstester/commit/297bf2795c4bc62c55de121b47e0a1bb62ad6108).

### `eksconfig`

- Add [`eksconfig.Instance.LaunchTime`](https://github.com/aws/awstester/commit/d886cbeb0d7ea9b8e71f0b9bf57e04923985202d) field.

### `internal`

- Add [`"install-kubeadm"` plugin to `internal/ec2/config/plugins`](https://github.com/aws/awstester/commit/e103c1ca68742bb56a8c43d3508d0c09423bb6b5).
- Add [`internal/ec2/config.Config.InitScriptCreated`](https://github.com/aws/awstester/commit/793935db2418a7c960d89512372f534996adcb19) flag.
- Add [`internal/ec2/config.Instance.LaunchTime`](https://github.com/aws/awstester/commit/36fe5579ffb719d108272640c22f478127295dac) field.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>


## [0.1.0](https://github.com/aws/awstester/releases/tag/0.1.0) (2018-10-29)

See [code changes](https://github.com/aws/awstester/compare/0.0.9...0.1.0).

### `awstester` CLI

- Add [`awstester eks prow status-get --data-dir`](https://github.com/aws/awstester/commit/034b9f6667b664368bace942b2e8f160c1eadf9f) flag.

### Go

- Compile with [*Go 1.11.1*](https://golang.org/doc/devel/release.html#go1.11).


<hr>

