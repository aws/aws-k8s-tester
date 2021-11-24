# How we added/built stand-alone tests for SecureCN

- git clone 
- `mkdir/secureCN`
- `cd secureCN/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/secureCN`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`


Test/Run single test stand-alone
```bash
go run cmd/k8-tester-secureCN/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --access-key=<access key> --secret-key=<secret key> \
    --URL=<url> --ClusterName=<cluster name>
## Delete
go run cmd/k8-tester-secureCN/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig"
```