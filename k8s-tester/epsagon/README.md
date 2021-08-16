
# How we added/built stand-alone tests for Epsagon

- git clone 
- `mkdir/epsagon`
- `cd epsagon/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/epsagon`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`



Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-epsagon/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/go/src/github.com/eks-anywhere/jonahjo2.kubeconfig" \
    --log-outputs="epsagon.log" \
    --api-token="6e75f4ad-7d99-4268-a3dc-92972838fbbd" \
    --collector-endpoint="https://collector.epsagon.com/ingestion?6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].basic_auth.username=6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].target_label=cluster_name,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].replacement=epsagon-application-cluster" \
    --cluster-name="epsagon-application-cluster" 

## Delete
go run cmd/k8s-tester-epsagon/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/PATHTO/kubeconfig" \
    --log-outputs="epsagon.log" \
    --api-token="6e75f4ad-7d99-4268-a3dc-92972838fbbd" \
    --collector-endpoint="https://collector.epsagon.com/ingestion?6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].basic_auth.username=6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].target_label=cluster_name,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].replacement=epsagon-application-cluster" \
    --cluster-name="epsagon-application-cluster" 
```


## Tests are equivilant to
```
helm repo add epsagon https://helm.epsagon.com
helm repo update
helm install epsagon-agent \
    --set epsagonToken="6e75f4ad-7d99-4268-a3dc-92972838fbbd" \
    --set clusterName="epsagon-application-cluster" \
    --set metrics.enabled=true \
    --set "metrics-agent.server.remoteWrite[0].url=https://collector.epsagon.com/ingestion?6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].basic_auth.username=6e75f4ad-7d99-4268-a3dc-92972838fbbd,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].target_label=cluster_name,metrics-agent.server.remoteWrite[0].write_relabel_configs[0].replacement=epsagon-application-cluster" \
    epsagon/cluster-agent
    
```