# Nvidia AI/ML Test Suite for EKS

This suite provides tools and scripts for running Nvidia AI/ML tests on Amazon EKS.

## Usage

### Build the Nvidia Base Image

```bash
docker build -t nvidia . -f e2e2/test/images/nvidia/Dockerfile
```

### Tag and Push the Image to the Repository

```bash
docker tag nvidia:latest <image-repo-url>:nvidia
docker push <image-repo-url>:nvidia
```

### Build the Kubetest2 Image

```bash
docker build --file Dockerfile.kubetest2 --build-arg KUBERNETES_MINOR_VERSION=1.30 --build-arg TARGETOS="linux" --build-arg TARGETARCH="amd64" -t kubetest2 .
```

### Enter the Kubetest2 Container

```bash
docker run --name kubetest2 -d -i -t kubetest2 /bin/sh
docker exec -it kubetest2 sh
```

### Set the AWS Credentials in the Container

```bash
export AWS_ACCESS_KEY_ID=xxxxxx
export AWS_SECRET_ACCESS_KEY=xxxxxx
export AWS_SESSION_TOKEN=xxxxxx
```

### Create the K8s Test Cluster and Run the Test

```bash
kubetest2 eksapi --kubernetes-version=<K8S_VERSION> --up --unmanaged-nodes --ami <AMI_ID> \
--instance-types p3dn.24xlarge --efa --generate-ssh-key --test=multi -- --fail-fast=true \
-- exec e2e-nvidia --test.timeout=60m --nvidiaTestImage=<IMAGE_URL> --test.v --efaEnabled=true
```

## Additional Flags

- `--nodeType` - Select the node type for the tests.
- `--efaEnabled` - Enable EFA in the test.
- `--region` - Specify the AWS region.
- `--ampMetricUrl` - Emit metrics to the AMP.
- `--ampMetricRoleArn` - Additional service principal that can assume the cluster IAM role.