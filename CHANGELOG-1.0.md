

<hr>


## [v1.0.8](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.8) (2020-04)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.7...v1.0.8).

### `eksconfig`

- Increase [`NGMaxLimit` from 100 to 300](https://github.com/aws/aws-k8s-tester/commit/6ba51a14f4689996b326c001e6045bbde8306274).

### `ec2config`

- Rename [`ASG.SSMDocumentCommandID` to `ASG.SSMDocumentCommandIDs` as `[]string` type](https://github.com/aws/aws-k8s-tester/commit/7aed1aa60a370c5cf924357b3d197e60d04c1b92).

### `ec2`

- Replace [`httpDownloadFile` with `httputil.Download`](https://github.com/aws/aws-k8s-tester/commit/).
- Replace [`httpReadInsecure` with `httputil.ReadInsecure`](https://github.com/aws/aws-k8s-tester/commit/).
- Batch [SSM execution by 50](https://github.com/aws/aws-k8s-tester/commit/7aed1aa60a370c5cf924357b3d197e60d04c1b92).
  - Fix `'instanceIds' failed to satisfy constraint: Member must have length less than or equal to 50`.

### `eks`

- Add [`"github.com/aws/aws-k8s-tester/eks/kubeflow"`](https://github.com/aws/aws-k8s-tester/commit/).
- Improve [node waits using node labels](https://github.com/aws/aws-k8s-tester/commit/7aed1aa60a370c5cf924357b3d197e60d04c1b92).
- Batch [SSM execution by 50](https://github.com/aws/aws-k8s-tester/commit/7aed1aa60a370c5cf924357b3d197e60d04c1b92).
  - Fix `'instanceIds' failed to satisfy constraint: Member must have length less than or equal to 50`.

### Dependency

- Upgrade [`github.com/helm/helm`](https://github.com/helm/helm/releases) from [`v3.2.0-rc.1`](https://github.com/helm/helm/releases/tag/v3.2.0-rc.1) to [`v3.2.0`](https://github.com/helm/helm/releases/tag/v3.2.0).
- Upgrade [`github.com/uber-go/zap`](https://github.com/uber-go/zap/releases) from [`v1.14.1`](https://github.com/uber-go/zap/releases/tag/v1.14.1) to [`v1.15.0`](https://github.com/uber-go/zap/releases/tag/v1.15.0).

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>



## [v1.0.7](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.7) (2020-04-23)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.6...v1.0.7).

### `eks`

- Add [`AddOnIRSAFargate` for "IAM Roles for Service Accounts (IRSA)" Pod with Fargate](https://github.com/aws/aws-k8s-tester/commit/a81b0245401cfdc51188dfaed1641bb39a77107d).
- Fix [`eks/fargate` `kubectl logs`](https://github.com/aws/aws-k8s-tester/commit/13749885f2f189f610e9d7fef99d89e04fde3793).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.11) to [`v1.30.13`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.13).

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.6](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.6) (2020-04-22)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.5...v1.0.6).

### `aws-k8s-tester`

- Add [`aws-k8s-tester eks create/delete --enable-prompt` flag](https://github.com/aws/aws-k8s-tester/commit/b83d6bd925c83592dd1ecc6cceb813de0d0442a2).
  - `--enable-prompt=true` by default.
  - `--enable-prompt=false` to disable.
- Add [`aws-k8s-tester ec2 create/delete --enable-prompt` flag](https://github.com/aws/aws-k8s-tester/commit/c7de973f5e411f1bd1237b89e9a0dc99c9b23ef7).
  - `--enable-prompt=true` by default.
  - `--enable-prompt=false` to disable.

### `ecconfig`

- Support [`GetRef.Name` in `Config.ASGs`](https://github.com/aws/aws-k8s-tester/commit/2629d4795cc423d6bed050fc89e6f0985844214a).
  - e.g. `{"GetRef.Name-ng-for-cni":{"name":"GetRef.Name-ng-for-cni","ssm-document-cfn-stack-name":"GetRef.Name-doc", "ssm-document-name":"GetRef.Name-name", "remote...`
- Automatically [fix invalid SSM document name in `ec2config.ASG.SSMDocumentName`](https://github.com/aws/aws-k8s-tester/commit/4914ca5d5fec4127932d9646a2885af8898baa6b).

### `eksconfig`

- Seperate [files for `AddOn*`](https://github.com/aws/aws-k8s-tester/commit/28d6baa83ce4df8e1c32b849c8e4d0ac5e3e3682).
- Rename [`AddOnAppMesh.AddOnCFNStackARN` to `AddOnAppMesh.PolicyCFNStackID`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`AddOnWordpress`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`AddOnCSRs.InitialRequestConditionType` for simulate an initial CSR condition](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Support [variable evaluation for `AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER` and `AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
  - `AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER="aws eks describe-cluster --name GetRef.Name"`
  - `AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS="echo GetRef.ClusterARN"`
- Add [`AddOnKubernetesDashboard`](https://github.com/aws/aws-k8s-tester/commit/e07dedcf6dc2be7837f6a1a78fd7e37024ba17d8).
- Add [`AddOnPrometheusGrafana`](https://github.com/aws/aws-k8s-tester/commit/115da16e9f9887dc71998ff6940cf5908f082af9).
- Add [`AddOnCSIEBS`](https://github.com/aws/aws-k8s-tester/commit/bd343e016ac8be7b912985c2972eb75361ac1599).
- Fix [`AddOnFargate.ProfileName` reserved prefix validation check](https://github.com/aws/aws-k8s-tester/commit/5a032a78be4e8daf2a6325ba3889c2fb3e752eb0).
- Support [`GetRef.Name` in `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS` and `AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS`](https://github.com/aws/aws-k8s-tester/commit/2629d4795cc423d6bed050fc89e6f0985844214a).
  - e.g. `{"GetRef.Name-ng-for-cni":{"name":"GetRef.Name-ng-for-cni","ssm-document-cfn-stack-name":"GetRef.Name-doc", "ssm-document-name":"GetRef.Name-name", "remote...`
- Automatically [fix invalid SSM document name in `ec2config.ASG.SSMDocumentName`](https://github.com/aws/aws-k8s-tester/commit/4914ca5d5fec4127932d9646a2885af8898baa6b).

### `eks`

- Create [node labels and use `nodeSelector` for add-ons](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`NGType` node labels](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Rename [`"github.com/aws/aws-k8s-tester/eks/appmesh"` to `"github.com/aws/aws-k8s-tester/eks/app-mesh"`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Improve [`eks/app-mesh` to update helm repo before install](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Improve [`"github.com/aws/aws-k8s-tester/eks/csrs"` to simulate initial CSR conditions](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`"github.com/aws/aws-k8s-tester/eks/helm"`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`"github.com/aws/aws-k8s-tester/eks/wordpress"`](https://github.com/aws/aws-k8s-tester/commit/eb3368cbf7c3fa961fe600cfc89f5444f8a80b77).
- Add [`"github.com/aws/aws-k8s-tester/eks/kubernetes-dashboard"`](https://github.com/aws/aws-k8s-tester/commit/c296d0587a07112bf5c08f709e96f8d806d2828e).
- Add [`"github.com/aws/aws-k8s-tester/eks/prometheus-grafana"`](https://github.com/aws/aws-k8s-tester/commit/115da16e9f9887dc71998ff6940cf5908f082af9).
- Add [`"github.com/aws/aws-k8s-tester/eks/csi-ebs"`](https://github.com/aws/aws-k8s-tester/commit/bd343e016ac8be7b912985c2972eb75361ac1599).
- Add [retries to `aws eks update-kubeconfig`](https://github.com/aws/aws-k8s-tester/commit/bd343e016ac8be7b912985c2972eb75361ac1599).
- Fix [`AddOnIRSA` count success operation for `BOTTLEROCKET_x86_64` AMI](https://github.com/aws/aws-k8s-tester/commit/5a032a78be4e8daf2a6325ba3889c2fb3e752eb0).
- Add [`Name` tag to node group nodes](https://github.com/aws/aws-k8s-tester/commit/5a032a78be4e8daf2a6325ba3889c2fb3e752eb0).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.7`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.7) to [`v1.30.11`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.11).
- Upgrade [`github.com/helm/helm`](https://github.com/helm/helm/releases) from [`v3.1.1`](https://github.com/helm/helm/releases/tag/v3.1.1) to [`v3.2.0-rc.1`](https://github.com/helm/helm/releases/tag/v3.2.0-rc.1).
- Upgrade [`github.com/kubernetes/client-go`](https://github.com/kubernetes/client-go/releases) from [`kubernetes-1.15.4`](https://github.com/kubernetes/client-go/releases/tag/kubernetes-1.15.4) to [`v0.18.2`](https://github.com/kubernetes/client-go/releases/tag/v0.18.2).

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.5](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.5) (2020-04-15)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.4...v1.0.5).

### `etcd-utils`

- Add [`etcd-utils k8s list` `--csv-aggregated-ids` and `--csv-aggregated-output` flags](https://github.com/aws/aws-k8s-tester/commit/7b3975f12f2cca9da9927bd930d189119ccadf0a).

### `eks`

- Fix [node group checks when DHCP option domain name is set](https://github.com/aws/aws-k8s-tester/commit/e3ee6cce0d81b85f52aae3264384445e2c022f2d).
  - e.g. `"caller":"ng/asg.go:809","msg":"node may not belong to this ASG","host-name":"ip-192-168-132-188.my-private-dns"`

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.4](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.4) (2020-04-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.3...v1.0.4).

### `etcd-utils`

- Add [`etcd-utils k8s list --csv-ids` flag](https://github.com/aws/aws-k8s-tester/commit/cb661c3339fd31b39c8e315028fc97fe1e73ca56).
  - Read https://github.com/aws/aws-k8s-tester#etcd-utils-k8s-list.

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.3](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.3) (2020-04-14)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.2...v1.0.3).

### `etcd-utils`

- Add [`etcd-utils k8s list`](https://github.com/aws/aws-k8s-tester/commit/86ba0378fa0125b9dda552697dd3cb717be13f7a).
  - Read https://github.com/aws/aws-k8s-tester#etcd-utils-k8s-list.

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.2) (2020-04-13)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.1...v1.0.2).

### `eks-utils`

- Add [`eks-utils`](https://github.com/aws/aws-k8s-tester/commit/198631e1ae10ed4eb1a76346e27730290eb0675b).
  - Read https://github.com/aws/aws-k8s-tester#eks-utils-apis.

### `aws-k8s-tester`

- Remove [`aws-k8s-tester eks test`](https://github.com/aws/aws-k8s-tester/commit/237075a8130f1ad29e3c3b655ca4d52fa5632426).
- Improve [`aws-k8s-tester eks check`](https://github.com/aws/aws-k8s-tester/commit/237075a8130f1ad29e3c3b655ca4d52fa5632426).
  - [`aws-k8s-tester eks check cluster` is now just `aws-k8s-tester eks check`](https://github.com/aws/aws-k8s-tester/commit/623350901946156b97ef985aa4b2344a9e654835).

### `eksconfig`

- Rename [`eksconfig.Parameters.ControlPlaneSecurityGroupID` to `eksconfig.Status.ClusterControlPlaneSecurityGroupID`](https://github.com/aws/aws-k8s-tester/commit/14565868ed452f6d9ffa8335935192bcb0d42e86).
  - Does not break anything, since `ControlPlaneSecurityGroupID` was a read-only field.
- Add [`eksconfig.Status.(k8s.io/apimachinery/pkg/version).Info` as `Status.ServerVersionInfo`](https://github.com/aws/aws-k8s-tester/commit/ba7231019be4637e0bbbd91220b260e903ecb5b6).
- Include [`float64` version value in `ServerVersionInfo`](https://github.com/aws/aws-k8s-tester/commit/ba7231019be4637e0bbbd91220b260e903ecb5b6).

### `eks`

- Use [`pkg/k8s-client.NewEKS` for `*kubernetes.Clientset`; use `pkg/k8s-client.EKS` interface](https://github.com/aws/aws-k8s-tester/commit/85db2dd0c9f64af5d37be1b304d63ff2d42cdc79).
- Move [`healthz` checks to `pkg/k8s-client.EKS` interface](https://github.com/aws/aws-k8s-tester/commit/0d7981d66303ba8384ec57b338feb084bca64bdf).
- Fix [node group instance check when `DHCP` options are set](https://github.com/aws/aws-k8s-tester/commit/2cc88ee4ab04159ec04306400f7f5d8c44b81f8d).
- Log [node `Labels` when polling node status](https://github.com/aws/aws-k8s-tester/commit/26f67f5285ffdb748914233272857bb7ff0f048e).
- Open [`30000-32767` ports for node group](https://github.com/aws/aws-k8s-tester/commit/bcc27696b8d2d1524db78faec64ec4bf3ad601a0).
  - `NodePort` conformance test requires `30000-32767` ports to be open from nodes to internet, request to node over public IP in those range.
  - https://github.com/kubernetes/kubernetes/blob/release-1.16/test/e2e/network/service.go#L544.
- Use [CloudFormation stack to create security group for managed node group](https://github.com/aws/aws-k8s-tester/commit/9e4601335b290dc145e0f137c5d12e1d58989e47).
- Rename [`eksconfig.Parameters.ClusterControlPlaneSecurityGroupID` to `eksconfig.Status.ClusterControlPlaneSecurityGroupID`](https://github.com/aws/aws-k8s-tester/commit/14565868ed452f6d9ffa8335935192bcb0d42e86).
- Fetch [server version in health check](https://github.com/aws/aws-k8s-tester/commit/720f4598b21ec7fe2cfff56e8eda128fc0056996).
- Highlight [errors if `Up` fails](https://github.com/aws/aws-k8s-tester/commit/720f4598b21ec7fe2cfff56e8eda128fc0056996).

### `pkg/k8s-client`

- Add [`k8sclient.NewEKS` and `k8sclient.EKSConfig` for `*kubernetes.Clientset`; use `pkg/k8s-client.EKS` interface](https://github.com/aws/aws-k8s-tester/commit/e673d3388ee44889e6572dcdcee530ea06984a86).
- Move [`healthz` checks to `k8sclient.EKS` interface](https://github.com/aws/aws-k8s-tester/commit/3dac533adcf2fb0aa51f19d4f56bbc9dd2b59eb5).
- Add [`k8sclient.EKS.FetchServerVersion`](https://github.com/aws/aws-k8s-tester/commit/56cd2d0f26e88f8c806a74a503def91769a3e8e3).
- Include [`float64` version value in `ServerVersionInfo`](https://github.com/aws/aws-k8s-tester/commit/ba7231019be4637e0bbbd91220b260e903ecb5b6).

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.1) (2020-04-08)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.0.0...v1.0.1).

### `ec2config`

- Add [`DHCPOptionsDomainName`](https://github.com/aws/aws-k8s-tester/commit/1f90891e0aeaa9fcffb25acda12f5f4e4a78f706).
  - `AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME`
- Add [`DHCPOptionsDomainNameServers`](https://github.com/aws/aws-k8s-tester/commit/1f90891e0aeaa9fcffb25acda12f5f4e4a78f706).
  - `AWS_K8S_TESTER_EC2_DHCP_OPTIONS_DOMAIN_NAME_SERVERS`

### `eksconfig`

- Add [`Parameters.DHCPOptionsDomainName`](https://github.com/aws/aws-k8s-tester/commit/84dd682a673eaa01fbf6bbbf3e664ad82c1dbbf4
).
  - `AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME`
- Add [`Parameters.DHCPOptionsDomainNameServers`](https://github.com/aws/aws-k8s-tester/commit/84dd682a673eaa01fbf6bbbf3e664ad82c1dbbf4).
  - `AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME_SERVERS`
- Change [`eksconfig.Config.AddOnNodeGroups.ASGs` from `map[string]ec2config.ASG` to `map[string]eksconfig.ASG`](https://github.com/aws/aws-k8s-tester/commit/e302d15f428e014931e1f43a3a0e8cafec136e77).
  - To support `--kubelet-extra-args`.
  - Added `eksconfig.ASG` with `KubeletExtraArgs` field.
  - ref. https://github.com/awslabs/amazon-eks-ami/blob/master/files/bootstrap.sh

### `eks`

- Improve [`AddOnNodeGroups` delete operation](https://github.com/aws/aws-k8s-tester/commit/90b0b50819da58a48cfebef8f6172238426dd8b5).
- Improve [`AddOnManagedNodeGroups` delete operation](https://github.com/aws/aws-k8s-tester/commit/5a21706eaf6ff00b65ef651385b99b6f23676633).

### Dependency

- Upgrade [`github.com/aws/aws-sdk-go`](https://github.com/aws/aws-sdk-go/releases) from [`v1.30.4`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.4) to [`v1.30.7`](https://github.com/aws/aws-sdk-go/releases/tag/v1.30.7).

### Go

- Compile with [*Go 1.14.2*](https://golang.org/doc/devel/release.html#go1.14).


<hr>


## [v1.0.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.0.0) (2020-04-05)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v0.9.8...v1.0.0).

### `ec2config`

- Update [`README.md`](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Clean up [`RemoteAccessPrivateKeyPath` defaults](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Fix [`ec2config.NewDefault`, remove `DefaultConfig`](https://github.com/aws/aws-k8s-tester/commit/13eabf5034488eefa0a028449f3f23233ef74661).
  - `ec2config.NewDefault` was copying the add-on fields in reference.
- Check [`ImageID` and `ImageIDSSMParameter`](https://github.com/aws/aws-k8s-tester/commit/13151dfd539a31175a9014e2115148605c9bc001).

### `ec2`

- Improve [ASG create and delete performance](https://github.com/aws/aws-k8s-tester/commit/4a97173663a4f383b2810051fd630b93c49f6351).

### `eksconfig`

- Update [`README.md`](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Clean up [`RemoteAccessPrivateKeyPath` defaults](https://github.com/aws/aws-k8s-tester/commit/eb0d6bca8bd01da418901acfa2c7b1fd5080d9bd).
- Fix [`eksconfig.NewDefault`, remove `DefaultConfig`](https://github.com/aws/aws-k8s-tester/commit/13eabf5034488eefa0a028449f3f23233ef74661).
  - `eksconfig.NewDefault` was copying the add-on fields in reference.
- Check [`ImageID` and `ImageIDSSMParameter`](https://github.com/aws/aws-k8s-tester/commit/13151dfd539a31175a9014e2115148605c9bc001).

### `eks`

- Add [missing `AddOnCSRs` delete operation](https://github.com/aws/aws-k8s-tester/commit/e91e12f256a60d74a9f08dead964608f74beee5a).
- Add [missing `AddOnConfigMaps` delete operation](https://github.com/aws/aws-k8s-tester/commit/e91e12f256a60d74a9f08dead964608f74beee5a).
- Improve [inflight creation requests cancel](https://github.com/aws/aws-k8s-tester/commit/da59e6bca6c117b3737bbb36598a3830b63ec7cf).
- Upgrade [`eks/alb` `kubernetes-sigs/aws-alb-ingress-controller` version from `v1.1.5` to `v1.1.6`](https://github.com/aws/aws-k8s-tester/commit/8df3fc79196113d19ad84077aab3bdc1c3805249).
- Delete [encryption CMK at the end](https://github.com/aws/aws-k8s-tester/commit/2436dafee14582014a97a08637272211c80f1d79).
  - Otherwise, `kube-apiserver` `/healthz` check fails.

### `pkg/k8s-client`

- Increase [`DefaultNamespaceDeletionInterval` from 5-second to 15-second](https://github.com/aws/aws-k8s-tester/commit/1a41c61813e1e0872b44738773ccdda4e765be1c).
- Improve [`DeleteNamespaceAndWait` retries on `i/o timeout`](https://github.com/aws/aws-k8s-tester/commit/1a41c61813e1e0872b44738773ccdda4e765be1c).

### Dependency

- Upgrade [`github.com/go-ini/ini`](https://github.com/go-ini/ini/releases) from [`v1.46.0`](https://github.com/go-ini/ini/releases/tag/v1.46.0) to [`v1.55.0`](https://github.com/go-ini/ini/releases/tag/v1.55.0).
- Upgrade [`sigs.k8s.io/yaml`](https://github.com/kubernetes-sigs/yaml/releases) from [`v1.1.0`](https://github.com/kubernetes-sigs/yaml/releases/tag/v1.1.0) to [`v1.2.0`](https://github.com/kubernetes-sigs/yaml/releases/tag/v1.2.0).

### Go

- Compile with [*Go 1.14.1*](https://golang.org/doc/devel/release.html#go1.14).


<hr>

