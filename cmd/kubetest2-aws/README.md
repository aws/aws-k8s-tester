# kubetest2-aws

This directory contains source code to build kubetest2-aws to provison and delete eks clusters that can act as a deployer for kubetest2.

## Commands
Run the following commands from the top level of repo

Build kubetest2-aws binary in bin. 

```
make deployer
```

Install kubetest2-aws to $GOPATH/bin

```
make install
```

### Useful enviroment variables

`AWS_K8S_TESTER_EKS_CONFIG_INPUT` - Path to the eks configuration as explained in the repo README.md

`AWS_K8S_TESTER_EKS_VERSION` - K8s major version

`AWS_K8S_TESTER_EKS_NAME` - Cluster name, by default if `AWS_K8S_TESTER_EKS_CONFIG_INPUT` is not set a config with the $(AWS_K8S_TESTER_EKS_NAME).yml will be created in /tmp