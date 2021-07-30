# How we added/built stand-alone tests for Kubecost

- git clone 
- `mkdir/kubecost`
- `cd kubecost/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/kubecost`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`


# Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-kubecost/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/Username/.kube/config" \
    --log-outputs="kubecost.log"


## Delete
go run cmd/k8s-tester-kubecost/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/Username/.kube/config" \
    --log-outputs="kubecost.log"

```

# This test is equivilant to running this set of commands
kubectl create namespace kubecost
helm repo add https://kubecost.github.io/cost-analyzer/
helm install kubecost cost-analyzer --namespace kubecost --set --set persistentVolume.enabled="false" --set prometheus.server.persistentVolume.enabled=false

# After running apply how to check kubecost
kubectl port-forward --namespace kubecost deployment/cost-analyzer 9090





