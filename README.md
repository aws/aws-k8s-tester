# Tools for testing Kubernetes on AWS

## Installation

This project will use rolling releases going forward; we recommend fetching the latest commit:
```
go install github.com/aws/aws-k8s-tester/...@HEAD
```

You'll need the standard `kubetest` tools as well:
```
go install sigs.k8s.io/kubetest2/...@latest
```

## `kubetest2` deployers and testers for EKS


### Usage

**Auto-detect cluster version**

The deployers will search for a file called `kubernetes-version.txt` on your `PATH`.
This file should contain a valid tag for a Kubernetes release.
The `--kubernetes-version` flag can be omitted if this file exists.

---

### `eksctl` deployer

This deployer is a thin wrapper around `eksctl`.

The simplest usage is:
```
kubetest2 \
  eksctl \
  --kubernetes-version=X.XX \
  --up \
  --down \
  --test=exec \
  -- echo "Hello world"
```

**Additional flags**

- `--instance-types` - comma-separated list of instance types to use for nodes
- `--ami` - AMI ID for nodes
- `--nodes` - number of nodes
- `--region` - AWS region
- `--config-file` - Path to eksctl config file (**if provided, other flags are ignored**)
- `--availability-zones` - Node availability zones
- `--ami-family` - AMI family to use: `AmazonLinux2023` | `Bottlerocket`
- `--efa-enabled` - Enable Elastic Fabric Adapter for the nodegroup
- `--volume-size` - Size of the node root volume in GB
- `--private-networking` - Use private networking for nodes
- `--with-oidc` - Enable OIDC provider for IAM roles for service accounts
- `--deploy-target` - The target to deploy: `cluster` | `nodegroup` (defaults to `cluster`)
- `--cluster-name` - Name of the EKS cluster (defaults to RunID if not specified)
- `--unmanaged-nodegroup` - Use unmanaged nodegroup instead of managed nodegroup
- `--nodegroup-name` - Name of the nodegroup (defaults to `ng-1`)

---

### `eksapi` deployer

This deployer calls the EKS API directly, instead of using CloudFormation for EKS resources.

The simplest usage is:
```
kubetest2 \
  eksapi \
  --kubernetes-version=X.XX \
  --up \
  --down \
  --test=exec \
  -- echo "Hello world"
```

**Additional flags**

- `--instance-types` - comma-separated list of instance types to use for nodes
- `--ami` - AMI ID for nodes
- `--nodes` - number of nodes
- `--region` - AWS region
- `--endpoint-url` - Override the EKS endpoint URL
- `--cluster-role-service-principal` - Additional service principal that can assume the cluster IAM role.

---

### `multi` tester

This tester wraps multiple executions of other testers.

Tester argument groups are separated by `--`, with the first group being passed to the `multi` tester itself.

The first positional argument of each subsequent group should be the name of a tester.

```
kubetest2 \
  noop \
  --test=multi \
  -- \
  --fail-fast=true \
  -- \
  ginkgo \
  --focus-regex='\[Conformance\]' \
  --parallel=4 \
  -- \
  exec \
  go test ./my/test/package
```
