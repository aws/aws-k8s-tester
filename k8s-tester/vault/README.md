
# How we added/built stand-alone tests for Vault

- git clone 
- `mkdir/vault`
- `cd vault/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/vault`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`



Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-vault/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="vault.log" 

## Delete
go run cmd/k8s-tester-vault/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="vault.log" 
```

go run cmd/k8s-tester-vault/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/go/src/github.com/eks-anywhere/jonahjo2.kubeconfig" \
    --log-outputs="vault.log" 

go run cmd/k8s-tester-vault/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/go/src/github.com/eks-anywhere/jonahjo2.kubeconfig" \
    --log-outputs="vault.log" 



## Tests are equivilant to
```

helm repo add hashicorp https://helm.releases.hashicorp.com

helm repo update

helm install vault hashicorp/vault