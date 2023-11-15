# `kubetest2` deployers for EKS

### Installation

```
go install github.com/aws/aws-k8s-tester/kubetest2/...@latest
```

### Usage

#### `eksctl` deployer

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

**Auto-detect cluster version**

The deployer will search for a file called `kubernetes-version.txt` on your `PATH`.
This file should contain a valid tag for a Kubernetes release.
The `--kubernetes-version` flag can be omitted if this file exists.

**Additional flags**

- `--instance-types` - comma-separated list of instance types to use for nodes
- `--ami` - AMI ID for nodes
- `--nodes` - number of nodes
- `--region` - AWS region