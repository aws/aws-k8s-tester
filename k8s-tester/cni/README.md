
# How we added/built stand-alone tests for Falco

- git clone 
- `mkdir/cni`
- `cd cni/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/cni`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`



Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-cni/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/.kube/config" \
    --log-outputs="cni.log"

## Delete
go run cmd/k8s-tester-cni/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/.kube/config" \
    --log-outputs="cni.log"
```
