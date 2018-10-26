


WARNING: Pre-alpha. Do not use this in production. Only for testing.



# awstester

[![Go Report Card](https://goreportcard.com/badge/github.com/aws/awstester)](https://goreportcard.com/report/github.com/aws/awstester)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://godoc.org/github.com/aws/awstester)
[![Releases](https://img.shields.io/github/release/aws/awstester/all.svg?style=flat-square)](https://github.com/aws/awstester/releases)
[![LICENSE](https://img.shields.io/github/license/aws/awstester.svg?style=flat-square)](https://github.com/aws/awstester/blob/master/LICENSE)

## `awstester eks`

To install

```bash
go install -v ./cmd/awstester
awstester eks create cluster -h
```

To create an EKS testing cluster with ALB Ingress Controller

```bash
awstester eks create config --path ./awstester-eks.yaml

# change default configurations
vi ./awstester-eks.yaml
```

```bash
awstester eks create cluster --path ./awstester-eks.yaml
```

This will create an EKS cluster with ALB Ingress Controller (takes about 20 minutes).

Once cluster is created, check cluster state using AWS CLI:

```bash
aws eks describe-cluster \
  --name awstester-20180928-efeaantamazonco-Os0xhhKodH \
  --query cluster.status

"ACTIVE"
```

Cluser states are persisted on disk as well. EKS tester uses this file to track status.

```bash
cat ./awstester-eks.yaml
```

Once complete, get the DNS names from `./awstester-eks.yaml`.

And `curl` the `kube-system` namespace's `/metrics` endpoint, to see if it works.

```bash
# for example
curl -L http://e5de0f6b-kubesystem-ingres-6aec-38954145.us-west-2.elb.amazonaws.com/metrics
```

Tear down the cluster (takes about 10 minutes):

```bash
awstester eks delete cluster --path ./awstester-eks.yaml
```

### `awstester eks` e2e tests

To run locally:

```bash
aws ecr create-repository --repository-name awstester
aws ecr list-images --repository-name awstester

cd ${GOPATH}/src/github.com/aws/awstester
go install -v ./cmd/awstester

cd ${GOPATH}/src/github.com/aws/awstester
./scripts/awstester.build.container.push.sh


cd ${GOPATH}/src/github.com/aws/awstester

AWSTESTER_EKS_KUBETEST_EMBEDDED_BINARY=true \
  AWSTESTER_EKS_WAIT_BEFORE_DOWN=1m \
  AWSTESTER_EKS_DOWN=true \
  AWSTESTER_EKS_ENABLE_WORKER_NODE_HA=true \
  AWSTESTER_EKS_ENABLE_NODE_SSH=true \
  AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE=m3.xlarge \
  AWSTESTER_EKS_WORKER_NODE_ASG_MIN=1 \
  AWSTESTER_EKS_WORKER_NODE_ASG_MAX=1 \
  AWSTESTER_EKS_ALB_ENABLE=true \
  AWSTESTER_EKS_ALB_TARGET_TYPE=instance \
  AWSTESTER_EKS_ALB_TEST_MODE=nginx \
  AWSTESTER_EKS_ALB_TEST_SCALABILITY=true \
  AWSTESTER_EKS_ALB_TEST_METRICS=true \
  AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS=1 \
  AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES=1 \
  AWSTESTER_EKS_ALB_TEST_CLIENTS=20 \
  AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS=2000 \
  AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE=40960 \
  AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD=500 \
  AWSTESTER_EKS_ALB_TEST_EXPECT_QPS=100 \
  AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE=quay.io/coreos/alb-ingress-controller:1.0-beta.7 \
  ./tests/ginkgo.sh
```
