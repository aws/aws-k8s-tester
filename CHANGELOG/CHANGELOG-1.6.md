
<hr>

## [v1.6.2](https://github.com/aws/aws-k8s-tester/releases/tag/v1.6.2) (2021-09-15)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.6.1...v1.6.2).

### `aws-k8s-tester`

- Update `helm` and `kubernetes` dependency.
  - https://github.com/kubernetes/kubernetes/releases/tag/v1.21.1
  - https://github.com/helm/helm/releases/tag/v3.6.3
  - https://github.com/aws/aws-k8s-tester/pull/240

### `k8s-tester`

- Update `helm` and `kubernetes` dependency.
  - https://github.com/kubernetes/kubernetes/releases/tag/v1.21.1
  - https://github.com/helm/helm/releases/tag/v3.6.3
  - https://github.com/aws/aws-k8s-tester/pull/240
- Add [`k8s-tester/aqua`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/aqua).
- Add [`k8s-tester/armory`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/armory).
- Add [`k8s-tester/cni`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/cni).
- Add [`k8s-tester/epsagon`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/epsagon).
- Add [`k8s-tester/kubecost`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/kubecost).
- Add [`k8s-tester/splunk`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/splunk).
- Add [`k8s-tester/sysdig`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/sysdig).
- Add [`k8s-tester/vault`](https://github.com/aws/aws-k8s-tester/tree/v1.6.2/k8s-tester/vault).
- See https://github.com/aws/aws-k8s-tester/blob/v1.6.2/k8s-tester/README.md.

### Go

- Compile with [*Go 1.17.1*](https://golang.org/doc/devel/release.html#go1.17).


<hr>

## [v1.6.1](https://github.com/aws/aws-k8s-tester/releases/tag/v1.6.1) (2021-07-19)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.6.0...v1.6.1).

### `eksconfig`

- Rename all fields [`AWS_K8S_TESTER_EKS_PARAMETERS_*` to `AWS_K8S_TESTER_EKS_`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
- Rename fields [`AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CIDR_BLOCK*` to `AWS_K8S_TESTER_EKS_VPC_CIDRS`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
- Rename fields [`AWS_K8S_TESTER_EKS_PARAMETERS_PUBLIC_SUBNET_CIDR_*` to `AWS_K8S_TESTER_EKS_VPC_PUBLIC_SUBNET_CIDRS`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
- Rename fields [`AWS_K8S_TESTER_EKS_PARAMETERS_PRIVATE_SUBNET_CIDR_*` to `AWS_K8S_TESTER_EKS_VPC_PRIVATE_SUBNET_CIDRS`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
- Rename fields [`AWS_K8S_TESTER_EKS_PARAMETERS_DHCP_OPTIONS_DOMAIN_NAME*` to `AWS_K8S_TESTER_EKS_VPC_DHCP_OPTIONS_*`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
- Change [`ssm*` fields in `AddOnNodeGroups` as nested](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
  - before, `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS='{"GetRef.Name-ng-for-cni":{"name":"GetRef.Name-ng-for-cni","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34,"image-id":"my-ami",  "ssm-document-create":true,   "instance-type":"type-2",  "ssm-document-cfn-stack-name":"GetRef.Name-ssm", "ssm-document-name":"GetRef.Name-document", "kubelet-extra-args":"aaa aa", "cluster-autoscaler": {"enable" : true}, "volume-size":500}}'`
  - after, `AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS='{"GetRef.Name-ng-for-cni":{"name":"GetRef.Name-ng-for-cni","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","asg-min-size":30,"asg-max-size":35,"asg-desired-capacity":34, "instance-type":"type-2", "image-id":"my-ami",  "ssm":{"document-create":true,    "document-name":"GetRef.Name-document"}, "kubelet-extra-args":"aaa aa", "cluster-autoscaler": {"enable" : true}, "volume-size":500}}'`
- Rename [`AddOnNodeGroups.ASG.InstanceTypes` to `InstanceType`](https://github.com/aws/aws-k8s-tester/commit/a4a3e3635466731519a38f411a1035318fecec59).
  - Rename `"instance-types"` to `"instance-type"`.
- [Remove CloudFormation dependency from `AddOnManagedNodeGroups`](https://github.com/aws/aws-k8s-tester/commit/50e5c1b302430dc863d0116ea15cc8e926ee0077).

### `k8s-tester`

- Add [`k8s-tester/csrs`](https://github.com/aws/aws-k8s-tester/commit/90ef22a2e6505189f998d1f6ed738fe05f73d56d).
- Add [`k8s-tester/clusterloader`](https://github.com/aws/aws-k8s-tester/commit/7b9113c21f440623ec01bdea5d81a74176100746).
- Add [`k8s-tester/stress`](https://github.com/aws/aws-k8s-tester/commit/310f44bc0da12ca093b02f74680b34131d6283a6).
- Add [`k8s-tester/stress/in-cluster`](https://github.com/aws/aws-k8s-tester/commit/e0b5fa0b0fb97851d86d268d093f4754617c638b).
- Add [`k8s-tester/falco`](https://github.com/aws/aws-k8s-tester/pull/221).
- Add [`k8s-tester/nlb-guestbook`](https://github.com/aws/aws-k8s-tester/commit/6c985cfabff769c020d2f1f131c4106607fa5d95).
- Add [`k8s-tester/wordpress`](https://github.com/aws/aws-k8s-tester/commit/b5a8f3e6533e199413269a27041aa70604318f57).

### Go

- Compile with [*Go 1.16.6*](https://golang.org/doc/devel/release.html#go1.16).


<hr>


## [v1.6.0](https://github.com/aws/aws-k8s-tester/releases/tag/v1.6.0) (2021-06-02)

See [code changes](https://github.com/aws/aws-k8s-tester/compare/v1.5.9...v1.6.0).

### Development

- Rename `master` branch to `main`.

### `client`

- Rename [`KubeConfig*` to `Kubeconfig*`](https://github.com/aws/aws-k8s-tester/commit/e7e10253a44a33ff9c16955a39df43d9e412c944).
- Automatically fetch [latest `kubectl` version](https://github.com/aws/aws-k8s-tester/commit/cfd76e8c53f444a3d3d1782a38801bb8d56baa49).

### `eks`

- Remove [`AmazonEKSServicePolicy` from default cluster role policy](https://github.com/aws/aws-k8s-tester/commit/8fe9e9b696333947b4420a3d08f72498e57d1766).
  - See https://docs.aws.amazon.com/eks/latest/userguide/service_IAM_role.html.
  - "Prior to April 16, 2020, AmazonEKSServicePolicy was also required and the suggested name was eksServiceRole. With the AWSServiceRoleForAmazonEKS service-linked role, that policy is no longer required for clusters created on or after April 16, 2020."

### `eksconfig`

- Set [default EKS version to 1.20](https://github.com/aws/aws-k8s-tester/commit/8f6b05193721b19378cdd1c69f6f2d787341d1f2).
- Add [`AddOnConformance.SonobuoyRunE2eFocus`](https://github.com/aws/aws-k8s-tester/pull/217).
  - Set via `AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_E2E_FOCUS`.
- Add [`AddOnConformance.SonobuoyRunE2eSkip`](https://github.com/aws/aws-k8s-tester/pull/217).
  - Set via `AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_SONOBUOY_RUN_E2E_SKIP`.

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
- Add [`k8s-tester/configmaps`](https://github.com/aws/aws-k8s-tester/commit/117fab905c090a3f8501112fc4885cc398f27db7).
- Add [`k8s-tester/secrets`](https://github.com/aws/aws-k8s-tester/commit/a77b8ceb473fe814bee5cb019f0df0c371185368).
- Add [`k8s-tester/conformance`](https://github.com/aws/aws-k8s-tester/commit/80c0b9e78252ab35cd8d58add52e8aee8615acc8).

### Dependency

- Upgrade [`go.uber.org/zap`](https://github.com/uber-go/zap/releases) from [`v1.16.0`](https://github.com/uber-go/zap/releases/tag/v1.16.0) to [`v1.17.0`](https://github.com/uber-go/zap/releases/tag/v1.17.0).

### Go

- Compile with [*Go 1.16.4*](https://golang.org/doc/devel/release.html#go1.16).


<hr>
