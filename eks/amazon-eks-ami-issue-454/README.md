This is intended to test a soft lockup issue described [here](https://github.com/awslabs/amazon-eks-ami/issues/454).
This based off of [this repo](https://github.com/mmerkes/eks-k8s-repro-assistant/tree/master/scenarios/decompression-loop).

### Running

Here's an example command that will run this test.

```
AWS_K8S_TESTER_EKS_ON_FAILURE_DELETE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ROLE_CREATE=true \
AWS_K8S_TESTER_EKS_ADD_ON_NODE_GROUPS_ASGS='{"soft-lockup":{"name":"soft-lockup","remote-access-user-name":"ec2-user","ami-type":"AL2_x86_64","image-id-ssm-parameter":"/aws/service/eks/optimized-ami/1.16/amazon-linux-2/recommended/image_id","instance-types":["m5.2xlarge"],"volume-size":40,"asg-min-size":1,"asg-max-size":1,"asg-desired-capacity":1,"kubelet-extra-args":"--node-labels amazon-ami-issue=454"}}' \
AWS_K8S_TESTER_EKS_ADD_ON_AMI_SOFT_LOCKUP_ISSUE_454_ENABLE=true \
AWS_K8S_TESTER_EKS_ADD_ON_AMI_SOFT_LOCKUP_ISSUE_454_DEPLOYMENT_NODE_SELECTOR='{"amazon-ami-issue":"454"}' \
AWS_K8S_TESTER_EKS_PARAMETERS_REQUEST_HEADER_KEY="x-eks-opts" \
./bin/aws-k8s-tester-latest-darwin-amd64 eks create cluster --enable-prompt=true --path ./stack/test.yaml
```
