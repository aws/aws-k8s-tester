
# How we added/built stand-alone tests for ondat

- git clone
- `mkdir/ondat`
- `cd ondat/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/ondat`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`

# Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-ondat/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Path/to/kubeconfig" \
    --etcd-replicas=3 \
    --etcd-storageclass=default \
    --log-outputs="ondat.log"

## Delete
go run cmd/k8s-tester-ondat/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Path/to/kubeconfig" \
    --etcd-replicas=3 \
    --etcd-storageclass=default \
    --log-outputs="ondat.log"
```
