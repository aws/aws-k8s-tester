

WARNING: Pre-alpha. Do not use this in production. Only for testing.


# aws-k8s-tester

[![Go Report Card](https://goreportcard.com/badge/github.com/aws/aws-k8s-tester)](https://goreportcard.com/report/github.com/aws/aws-k8s-tester)
[![Godoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](https://pkg.go.dev/github.com/aws/aws-k8s-tester)
[![Releases](https://img.shields.io/github/release/aws/aws-k8s-tester/all.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/releases)
[![LICENSE](https://img.shields.io/github/license/aws/aws-k8s-tester.svg?style=flat-square)](https://github.com/aws/aws-k8s-tester/blob/master/LICENSE)

`aws-k8s-tester` is a set of utilities and libraries for testing Kubernetes on AWS.

- Uses AWS CloudFormation for resource creation
- Supports automatic rollback and resource deletion
- Flexible add-on support via environmental variables
- Extensible as a Go package; `eks.Tester.Up` to create EKS

## Install

https://github.com/aws/aws-k8s-tester/releases


## `aws-k8s-tester ec2`

Make sure AWS credential is located in your machine:

```bash
# confirm credential is valid
aws sts get-caller-identity --query Arn --output text
```

See https://github.com/aws/aws-k8s-tester/blob/master/ec2config/README.md for more.

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text);
echo ${ACCOUNT_ID}

cd /tmp
rm -f /tmp/${USER}-test-ec2*
AWS_K8S_TESTER_EC2_ON_FAILURE_DELETE=true \
AWS_K8S_TESTER_EC2_NAME=${USER}-test-ec2 \
AWS_K8S_TESTER_EC2_REGION=us-west-2 \
AWS_K8S_TESTER_EC2_S3_BUCKET_NAME=aws-k8s-tester-ec2-s3-bucket \
AWS_K8S_TESTER_EC2_S3_BUCKET_CREATE=false \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_CREATE=true \
AWS_K8S_TESTER_EC2_REMOTE_ACCESS_KEY_NAME=aws-k8s-tester-ec2-key \
AWS_K8S_TESTER_EC2_ASGS_FETCH_LOGS=true \
AWS_K8S_TESTER_EC2_ASGS={\"${USER}-test-ec2-al2-cpu\":{\"name\":\"${USER}-test-ec2-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"AL2_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2\",\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40},\"${USER}-test-ec2-bottlerocket\":{\"name\":\"${USER}-test-ec2-bottlerocket\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"BOTTLEROCKET_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id\",\"ssm-document-name\":\"${USER}InstallBottleRocket\",\"ssm-document-create\":true,\"ssm-document-commands\":\"enable-admin-container\",\"ssm-document-execution-timeout-seconds\":3600,\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40}} \
AWS_K8S_TESTER_EC2_ROLE_CREATE=false \
AWS_K8S_TESTER_EC2_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/aws-k8s-tester-ec2-role \
AWS_K8S_TESTER_EC2_VPC_CREATE=false \
AWS_K8S_TESTER_EC2_VPC_ID=vpc-00219f2d3063b6d9c \
aws-k8s-tester ec2 create config -p /tmp/${USER}-test-ec2.yaml && cat /tmp/${USER}-test-ec2.yaml

# Or just run
aws-k8s-tester ec2 create config -p /tmp/${USER}-test-ec2.yaml
# to write initial configuration with default values


cd /tmp
aws-k8s-tester ec2 create cluster -p /tmp/${USER}-test-ec2.yaml

cd /tmp
aws-k8s-tester ec2 delete cluster -p /tmp/${USER}-test-ec2.yaml
```


## `aws-k8s-tester eks`

Make sure AWS credential is located in your machine:

```bash
# confirm credential is valid
aws sts get-caller-identity --query Arn --output text
```

See https://github.com/aws/aws-k8s-tester/blob/master/eksconfig/README.md for more.

```bash
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text);
CLUSTER_ARN=arn:aws:eks:us-west-2:${ACCOUNT_ID}:cluster/${USER}-test-eks
echo ${CLUSTER_ARN}

cd /tmp
rm -rf /tmp/${USER}-test-eks*
AWS_K8S_TESTER_EKS_NAME=${USER}-test-eks \
AWS_K8S_TESTER_EKS_REGION=us-west-2 \
AWS_K8S_TESTER_EKS_S3_BUCKET_NAME=aws-k8s-tester-eks-s3-bucket \
AWS_K8S_TESTER_EKS_S3_BUCKET_CREATE=false \
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_CREATE=false \
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_KEY_NAME=aws-k8s-tester-ec2-key \
AWS_K8S_TESTER_EKS_REMOTE_ACCESS_PRIVATE_KEY_PATH=./aws-k8s-tester-ec2-key.pem \
AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_CLUSTER="aws eks describe-cluster --name ${USER}-test-eks" \
AWS_K8S_TESTER_EKS_COMMAND_AFTER_CREATE_ADD_ONS="aws eks describe-cluster --name ${USER}-test-eks" \
AWS_K8S_TESTER_EKS_PARAMETERS_ENCRYPTION_CMK_CREATE=true \
AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_PARAMETERS_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/aws-k8s-tester-eks-role \
AWS_K8S_TESTER_EKS_PARAMETERS_VERSION=1.15 \
AWS_K8S_TESTER_EKS_PARAMETERS_VPC_CREATE=false \
AWS_K8S_TESTER_EKS_PARAMETERS_VPC_ID=vpc-0fd47acb73ace2208 \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/aws-k8s-tester-eks-role \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS={\"${USER}-test-eks-ng-al2-cpu\":{\"name\":\"${USER}-test-eks-ng-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"AL2_x86_64\",\"image-id-ssm-parameter\":\"/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended/image_id\",\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40}} \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_CREATE=false \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_ROLE_ARN=arn:aws:iam::${ACCOUNT_ID}:role/aws-k8s-tester-eks-role \
AWS_K8S_TESTER_EKS_ADD_ON_MANAGED_NODE_GROUPS_MNGS={\"${USER}-test-eks-mng-al2-cpu\":{\"name\":\"${USER}-test-eks-mng-al2-cpu\",\"remote-access-user-name\":\"ec2-user\",\"ami-type\":\"AL2_x86_64\",\"asg-min-size\":1,\"asg-max-size\":1,\"asg-desired-capacity\":1,\"instance-types\":[\"c5.xlarge\"],\"volume-size\":40}} \
AWS_K8S_TESTER_EKS_ADD_ON_NLB_HELLO_WORLD_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_ALB_2048_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOB_ECHO_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_JOB_PI_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_CRON_JOB_ENABLE=true \
aws-k8s-tester eks create config -p /tmp/${USER}-test-eks.yaml && cat /tmp/${USER}-test-eks.yaml

# Or just run
aws-k8s-tester eks create config -p /tmp/${USER}-test-eks.yaml
# to write initial configuration with default values


cd /tmp
aws-k8s-tester eks create cluster -p /tmp/${USER}-test-eks.yaml

cd /tmp
aws-k8s-tester eks delete cluster -p /tmp/${USER}-test-eks.yaml
```

This will create an EKS cluster with a worker node (takes about 20 minutes).

Once cluster is created, check cluster state using AWS CLI:

```bash
aws eks describe-cluster \
  --name ${USER}-test-eks \
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
