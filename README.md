

WARNING: Pre-alpha. Do not use this in production. Only for testing.


# aws-k8s-tester

[![Go Report Card](https://goreportcard.com/badge/github.com/aws/aws-k8s-tester)](https://goreportcard.com/report/github.com/aws/aws-k8s-tester)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/aws/aws-k8s-tester)
[![Releases](https://img.shields.io/github/release/aws/aws-k8s-tester/all.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/releases)
[![LICENSE](https://img.shields.io/github/license/aws/aws-k8s-tester.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/blob/master/LICENSE)

`aws-k8s-tester` is a set of utilities and libraries for testing Kubernetes on AWS.

## Install

https://github.com/aws/aws-k8s-tester/releases

## `aws-k8s-tester eks`

Make sure AWS credential is located in your machine:

```sh
cat ~/.aws/credentials

# confirm credential is valid
aws sts get-caller-identity --query Arn --output text
```

```bash
aws-k8s-tester eks create cluster -h

aws-k8s-tester eks create config --path /tmp/config.yaml
aws-k8s-tester eks create cluster --path /tmp/config.yaml
```

This will create an EKS cluster with a worker node (takes about 20 minutes).

Once cluster is created, check cluster state using AWS CLI:

```bash
aws eks describe-cluster \
  --name [NAME] \
  --query cluster.status

"ACTIVE"
```

Cluster states are persisted on disk as well. EKS tester uses this file to track status.

```bash
cat /tmp/config.yaml

# or
less +FG /tmp/config.yaml
```

Tear down the cluster (takes about 10 minutes):

```bash
aws-k8s-tester eks delete cluster --path /tmp/config.yaml
```
