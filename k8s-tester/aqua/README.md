
# How we added/built stand-alone tests for Aqua

- git clone 
- `mkdir/aqua`
- `cd aqua/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/aqua`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`



Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-aqua/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="aqua.log" \
    --aqua-license="1234567890" \
    --aqua-username="Username" \
    --aqua-password="Password" 

## Delete
go run cmd/k8s-tester-aqua/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="aqua.log" \
    --aqua-license="1234567890" \
    --aqua-username="Username" \
    --aqua-password="Password" 
```

## Tests are equivilant to
```

helm repo add aqua https://helm.aquasec.com

helm repo update

helm upgrade --install --namespace aqua aqua . --set ke.aquasecret.kubeEnforcerToken=12345 --set imageCredentials.username=12345@gmail.com --set imageCredentials.password=12345 --set web.service.type=ClusterIP