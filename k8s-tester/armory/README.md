
# How we added/built stand-alone tests for Armory

- git clone 
- `mkdir/falco`
- `cd falco/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/falco`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`



Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-armory/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="armory.log"

## Delete
go run cmd/k8s-tester-armory/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="armory.log"
```


## Tests are equivilant to
```
helm repo add armory https://armory.jfrog.io/artifactory/charts/
helm repo update
helm install armory --wait armory/armory
```