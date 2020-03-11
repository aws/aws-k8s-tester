// Package workernodes implements EKS worker nodes with a custom AMI.
package workernodes

/*
https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html
https://github.com/awslabs/amazon-eks-ami/blob/master/amazon-eks-nodegroup.yaml
https://raw.githubusercontent.com/awslabs/amazon-eks-ami/master/amazon-eks-nodegroup.yaml

https://aws.amazon.com/about-aws/whats-new/2019/09/amazon-eks-provides-eks-optimized-ami-metadata-via-ssm-parameters/

e.g.
/aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2

e.g.
/aws/service/eks/optimized-ami/1.15/amazon-linux-2/recommended
/aws/service/bottlerocket/aws-k8s-1.15/x86_64/latest/image_id
*/
