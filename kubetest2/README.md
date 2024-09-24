# `kubetest2` deployers and testers for EKS

### Installation

```
go install github.com/aws/aws-k8s-tester/kubetest2/...@latest
```

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
- `--capacity-reservation` - Use capacity reservation for the unmanaged nodegroup. `--efa` must be set when using this flag

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