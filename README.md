
# aws-k8s-tester

[![Go Report Card](https://goreportcard.com/badge/github.com/aws/aws-k8s-tester)](https://goreportcard.com/report/github.com/aws/aws-k8s-tester)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/aws/aws-k8s-tester)
[![Releases](https://img.shields.io/github/release/aws/aws-k8s-tester/all.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/releases)
[![LICENSE](https://img.shields.io/github/license/aws/aws-k8s-tester.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/blob/master/LICENSE)

https://github.com/kubernetes/enhancements/blob/master/keps/provider-aws/20181126-aws-k8s-tester.md

`aws-k8s-tester` is a set of utilities and libraries for "testing" Kubernetes on AWS.

- Implements [`test-infra/kubetest2` interface](https://github.com/kubernetes/test-infra/tree/master/kubetest2).
- Uses AWS CloudFormation for resource creation.
- Supports automatic rollback and resource deletion.
- Flexible add-on support via environmental variables.
- Extensible as a Go package; `eks.Tester.Up` to create EKS.
- Performance tests suites.

The main goal is to create "temporary" EC2 instances or EKS clusters for "testing" purposes:

- Upstream conformance tests
  - https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/sig-cloud-provider/aws/eks/eks-periodics.yaml
  - https://github.com/kubernetes/test-infra/pull/16890
- CNI plugin conformance tests
  - https://github.com/aws/amazon-vpc-cni-k8s/blob/master/scripts/lib/cluster.sh
  - https://github.com/aws/amazon-vpc-cni-k8s/pull/875
  - https://github.com/aws/amazon-vpc-cni-k8s/pull/878
  - https://github.com/aws/amazon-vpc-cni-k8s/pull/951
  - https://github.com/aws/amazon-vpc-cni-k8s/pull/957
- AppMesh scalability testing
  - https://github.com/aws/aws-app-mesh-controller-for-k8s/blob/master/scripts/lib/cluster.sh
  - https://github.com/aws/aws-app-mesh-controller-for-k8s/pull/137


## Install

https://github.com/aws/aws-k8s-tester/releases


## `aws-k8s-tester eks`

Make sure AWS credential is located in your machine:

```bash
# confirm credential is valid
aws sts get-caller-identity --query Arn --output text
```

See the following for more fields:
- https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/README.md
- https://pkg.go.dev/github.com/aws/aws-k8s-tester/eksconfig?tab=doc
- https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/default.yaml

```bash
# easiest way, use the defaults
# creates role, VPC, EKS cluster
rm -rf /tmp/${USER}-test-eks-prod*
aws-k8s-tester eks create cluster --enable-prompt=true -p /tmp/${USER}-test-prod-eks.yaml
aws-k8s-tester eks delete cluster --enable-prompt=true -p /tmp/${USER}-test-prod-eks.yaml

# advanced options can be set via environmental variables
# e.g. node groups, managed node groups, add-ons
rm -rf /tmp/${USER}-test-eks*
AWS_K8S_TESTER_EKS_PARTITION=aws \
AWS_K8S_TESTER_EKS_REGION=us-west-2 \
AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE=true \
AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE_KEEP=true \
AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER="aws eks describe-cluster --name GetRef.Name" \
AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS="aws eks describe-cluster --name GetRef.Name" \
AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_CREATE=true \
AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE=true \
AWS_K8S_TESTER_EKS_PARAMETERS_VERSION=1.16 \
AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE=true \
AWS_K8S_TESTER_EKS_CLIENTS=5 \
AWS_K8S_TESTER_EKS_CLIENT_QPS=30 \
AWS_K8S_TESTER_EKS_CLIENT_BURST=20 \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS='{"GetRef.Name-ng-al2-cpu":{"name":"GetRef.Name-ng-al2-cpu","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","image-id":"","image-id-ssm-parameter":"/aws/service/eks/optimized-ami/1.16/amazon-linux-2/recommended/image_id","instance-types":["c5.xlarge"],"volume-size":40,"asg-min-size":2,"asg-max-size":2,"asg-desired-capacity":2,"kubelet-extra-args":""},"GetRef.Name-ng-al2-gpu":{"name":"GetRef.Name-ng-al2-gpu","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64_GPU","image-id":"","image-id-ssm-parameter":"/aws/service/eks/optimized-ami/1.16/amazon-linux-2-gpu/recommended/image_id","instance-types":["p3.8xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1,"kubelet-extra-args":""},"GetRef.Name-ng-bottlerocket":{"name":"GetRef.Name-ng-bottlerocket","remote-access-user-name":"ec2-user","ami-type":"BOTTLEROCKET_x86_64","image-id":"","image-id-ssm-parameter":"/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id","ssm-document-cfn-stack-name":"GetRef.Name-install-bottlerocket","ssm-document-name":"GetRef.Name-InstallBottlerocket","ssm-document-create":true,"ssm-document-commands":"enable-admin-container","ssm-document-execution-timeout-seconds":3600,"instance-types":["c5.xlarge"],"volume-size":40,"asg-min-size":2,"asg-max-size":2,"asg-desired-capacity":2}}' \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE=true \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS='{"GetRef.Name-mng-al2-cpu":{"name":"GetRef.Name-mng-al2-cpu","remote-access-user-name":"ec2-user","release-version":"","ami-type":"AL2_x86_64","instance-types":["c5.xlarge"],"volume-size":40,"asg-min-size":2,"asg-max-size":2,"asg-desired-capacity":2},"GetRef.Name-mng-al2-gpu":{"name":"GetRef.Name-mng-al2-gpu","remote-access-user-name":"ec2-user","release-version":"","ami-type":"AL2_x86_64_GPU","instance-types":["p3.8xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1}}' \
AWS_K8S_TESTER_EKS_ADD_ON_CSI_EBS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_KUBERNETES_DASHBOARD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_PROMETHEUS_GRAFANA_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_APP_MESH_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOBS_PI_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOBS_ECHO_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOBS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CSRS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CONFIG_MAPS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_SECRETS_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_WORDPRESS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JUPYTER_HUB_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_HOLLOW_NODES_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CLUSTER_LOADER_LOCAL_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CONFORMANCE_ENABLE=true \
aws-k8s-tester eks create cluster --enable-prompt=true -p /tmp/${USER}-test-eks.yaml

<<COMMENT
# to delete
aws-k8s-tester eks delete cluster --enable-prompt=true -p /tmp/${USER}-test-eks.yaml

# run "eks create config" to check/edit configuration file first 
aws-k8s-tester eks create config -p /tmp/${USER}-test-eks.yaml

# run the following command with those envs overwrites configuration, and create
aws-k8s-tester eks create cluster --enable-prompt=true -p /tmp/${USER}-test-eks.yaml
COMMENT

<<COMMENT
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text);
echo ${ACCOUNT_ID}
CLUSTER_ARN=arn:aws:eks:us-west-2:${ACCOUNT_ID}:cluster/${USER}-test-eks
echo ${CLUSTER_ARN}

# to assign a non-random cluster name
# if empty, name is auto-generated
AWS_K8S_TESTER_EKS_NAME=${USER}-test-eks \

# to create/delete a S3 bucket for test artifacts
AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE=true \

# to reuse an existing S3 bucket
AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE=false \
AWS_K8S_TESTER_EKS_S3_BUCKET_NAME=${BUCKET_NAME} \

# to automatically create EC2 key-pair
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE=true \

# to reuse an existing EC2 key-pair
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE=false \
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME=${KEY_NAME} \
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH=${KEY_PATH} \

# to reuse an existing role for "EKS cluster"
AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_ARN=${CLUSTER_ROLE_ARN} \

# to reuse an existing VPC
AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE=false \
AWS_K8S_TESTER_EKS_PARAMETERS_VPC_ID=${VPC_ID} \

# to reuse an existing role for "Node Group"
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_ARN=${NG_ROLE_ARN} \

# to reuse an existing role for "Managed Node Group"
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_ARN=${MNG_ROLE_ARN} \

# to reuse an existing role for "Fargate"
AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_ADD_ON_FARGATE_ROLE_ARN=${FARGATE_ROLE_ARN} \

# to user ${USER} in node groups
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS={\"${USER}-test-eks-ng-al2-cpu\":{\"name\":\"${USER}-test-eks-ng-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"AL2_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id\",\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1,\"kubelet-extra-args\":\"\"},\"${USER}-test-eks-ng-bottlerocket\":{\"name\":\"${USER}-test-eks-ng-bottlerocket\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"BOTTLEROCKET_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id\",\"ssm-document-cfn-stack-name\":\"${USER}-install-bottle-rocket\",\"ssm-document-name\":\"${USER}InstallBottleRocket\",\"ssm-document-create\":true,\"ssm-document-commands\":\"enable-admin-container\",\"ssm-document-execution-timeout-seconds\":3600,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1}} \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS={\"${USER}-test-eks-mng-al2-cpu\":{\"name\":\"${USER}-test-eks-mng-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"release-version\":\"\",\"ami-type\":\"AL2_x86_64\",\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1}} \
COMMENT
```

This will create an EKS cluster with a worker node (takes about 20 minutes).

Once cluster is created, check cluster state using AWS CLI:

```bash
aws eks describe-cluster \
  --name ${EKS_CLUSTER_NAME} \
  --query cluster.status

"ACTIVE"
```

Cluster states are persisted on disk and S3 bucket.

EKS tester uses this file to record status.

```bash
cat /tmp/config.yaml

# or
less +FG /tmp/config.yaml
```


## `ec2-utils`

Make sure AWS credential is located in your machine:

```bash
# confirm credential is valid
aws sts get-caller-identity --query Arn --output text
```

See the following for more fields:
- https://github.com/aws/aws-k8s-tester/blob/master/ec2config/README.md
- https://pkg.go.dev/github.com/aws/aws-k8s-tester/ec2config?tab=doc
- https://github.com/aws/aws-k8s-tester/blob/master/ec2config/default.yaml

```bash
# easiest way, use the defaults
# creates role, VPC, EC2 ASG
rm -rf /tmp/${USER}-test-ec2*
ec2-utils create instances --enable-prompt=true -p /tmp/${USER}-test-ec2.yaml
ec2-utils delete instances --enable-prompt=true -p /tmp/${USER}-test-ec2.yaml

# advanced options can be set via environmental variables
rm -rf /tmp/${USER}-test-ec2*
AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE=true \
AWS_K8S_TESTER_EC2_PARTITION=aws \
AWS_K8S_TESTER_EC2_REGION=us-west-2 \
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE=true \
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE_KEEP=true \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE=true \
AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS=true \
AWS_K8S_TESTER_EC2_ASGS='{"GetRef.Name-al2-cpu":{"name":"GetRef.Name-al2-cpu","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","image-id-ssm-parameter":"/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2","instance-types":["c5.xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1},"GetRef.Name-bottlerocket":{"name":"GetRef.Name-bottlerocket","remote-access-user-name":"ec2-user","ami-type":"BOTTLEROCKET_x86_64","image-id-ssm-parameter":"/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id","ssm-document-cfn-stack-name":"GetRef.Name-install-bottlerocket","ssm-document-name":"GetRef.Name-install-bottlerocket","ssm-document-create":true,"ssm-document-commands":"enable-admin-container","ssm-document-execution-timeout-seconds":3600,"instance-types":["c5.xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1}}' \
AWS_K8S_TESTER_EC2_ROLE_CREATE=true \
AWS_K8S_TESTER_EC2_VPC_CREATE=true \
ec2-utils create instances --enable-prompt=true -p /tmp/${USER}-test-ec2.yaml

<<COMMENT
# to delete
ec2-utils delete instances --enable-prompt=true -p /tmp/${USER}-test-ec2.yaml

# run "ec2 create config" to check/edit configuration file first 
ec2-utils create config -p /tmp/${USER}-test-ec2.yaml
ec2-utils create instances -p /tmp/${USER}-test-ec2.yaml

# run the following command with those envs overwrites configuration, and create
ec2-utils create instances --enable-prompt=true -p /tmp/${USER}-test-ec2.yaml
COMMENT

<<COMMENT
# to config a fixed name for EC2 ASG
AWS_K8S_TESTER_EC2_NAME=${NAME} \

# to create/delete a S3 bucket for test artifacts
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE=true \

# to reuse an existing S3 bucket
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE=false \
AWS_K8S_TESTER_EC2_S3_BUCKET_NAME=${BUCKET_NAME} \

# to automatically create EC2 key-pair
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE=true \

# to reuse an existing EC2 key-pair
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE=false \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME=${KEY_NAME} \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_PRIVATE_KEY_PATH=${KEY_PATH} \

# to reuse an existing role
AWS_K8S_TESTER_EC2_ROLE_CREATE=false \
AWS_K8S_TESTER_EC2_ROLE_ARN=${ROLE_ARN} \

# to reuse an existing VPC
AWS_K8S_TESTER_EC2_VPC_CREATE=false \
AWS_K8S_TESTER_EC2_VPC_ID=${VPC_ID} \

# to use ${USER}
AWS_K8S_TESTER_EC2_ASGS={\"${USER}-test-ec2-al2-cpu\":{\"name\":\"${USER}-test-ec2-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"AL2_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2\",,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1},\"${USER}-test-ec2-bottlerocket\":{\"name\":\"${USER}-test-ec2-bottlerocket\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"BOTTLEROCKET_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id\",\"ssm-document-cfn-stack-name\":\"${USER}-install-bottlerocket\",\"ssm-document-name\":\"${USER}InstallBottleRocket\",\"ssm-document-create\":true,\"ssm-document-commands\":\"enable-admin-container\",\"ssm-document-execution-timeout-seconds\":3600,,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1}} \
COMMENT
```


## `eks-utils apis`

Install `eks-utils` from https://github.com/aws/aws-k8s-tester/releases.

```
AWS_K8S_TESTER_VERSION=${LATEST_RELEASE_VERSION}

DOWNLOAD_URL=https://github.com/aws/aws-k8s-tester/releases/download
rm -rf /tmp/aws-k8s-tester
rm -rf /tmp/eks-utils

if [[ "${OSTYPE}" == "linux"* ]]; then
  curl -L ${DOWNLOAD_URL}/${AWS_K8S_TESTER_VERSION}/eks-utils-${AWS_K8S_TESTER_VERSION}-linux-amd64 -o /tmp/eks-utils
elif [[ "${OSTYPE}" == "darwin"* ]]; then
  curl -L ${DOWNLOAD_URL}/${AWS_K8S_TESTER_VERSION}/eks-utils-${AWS_K8S_TESTER_VERSION}-darwin-amd64 -o /tmp/eks-utils
fi

chmod +x /tmp/eks-utils
/tmp/eks-utils version
```

`eks-utils apis` helps with API deprecation (e.g. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.16.md#deprecations-and-removals).

**WARNING**: `kubectl` internally converts API versions in the response (see [`kubernetes/issues#58131`](https://github.com/kubernetes/kubernetes/issues/58131#issuecomment-403829566)). Which means `kubectl get` output may have different API versions than the one persisted in `etcd` . Upstream Kubernetes recommends upgrading deprecated API with *get and put*:

> the simplest approach is to get/put every object after upgrades. objects that don't need migration will no-op (they won't even increment resourceVersion in etcd). objects that do need migration will persist in the new preferred storage version

Which means there's no way in client-side to find all resources created with deprecated API groups. The only way to ensure API group upgrades is list all resources, and execute *get and put* with the latest API group version. If the resource has already latest API version, it will be no-op. Otherwise, it will upgrade to the latest API version.

`eks-utils apis` will help with the list calls with proper pagination and generate *get and put* scripts for the cluster:

```bash
# to check supported API groups from current kube-apiserver
eks-utils apis \
  --kubeconfig /tmp/kubeconfig.yaml \
  supported

# to write API upgrade/rollback scripts and YAML files in "/tmp/eks-utils"
#
# make sure to set proper "--batch-limit" and "--batch-interval"
# to not overload EKS master; if it's set too high, it can affect
# production workloads slowing down kube-apiserver
rm -rf /tmp/eks-utils-resources
eks-utils apis \
  --kubeconfig /tmp/kubeconfig.yaml \
  --enable-prompt \
  deprecate \
  --batch-limit 10 \
  --batch-interval 2s \
  --dir /tmp/eks-utils-resources

# this command does not apply or create any resources
# it only lists the resources that need be upgraded

# if there's any resources that needs upgrade,
# it writes patched YAML file, original YAML file,
# bash scripts to update and rollback
find /tmp/eks-utils-resources
```

## `etcd-utils k8s list`

`etcd-utils k8s list` helps with API deprecation (e.g. https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.16.md#deprecations-and-removals).

**WARNING**: `kubectl` internally converts API versions in the response (see [`kubernetes/issues#58131`](https://github.com/kubernetes/kubernetes/issues/58131#issuecomment-403829566)). Which means `kubectl get` output may have different API versions than the one persisted in `etcd` . Upstream Kubernetes recommends upgrading deprecated API with *get and put*:

> the simplest approach is to get/put every object after upgrades. objects that don't need migration will no-op (they won't even increment resourceVersion in etcd). objects that do need migration will persist in the new preferred storage version

To minimize the impact of list calls, `etcd-utils k8s list` reads keys with leadership election and pagination; only a single worker can run at a time.

```bash
# to list all deployments with etcd pagination + k8s decoder
etcd-utils k8s \
  --endpoints http://localhost:2379 \
  list \
  --prefixes /registry/deployments \
  --output /tmp/etcd-utils-k8s-list.output.yaml

# or ".json"
```
