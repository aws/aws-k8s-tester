
# How we added/built stand-alone tests for Splunk

- git clone 
- `mkdir/falco`
- `cd falco/`
- `go mod init github.com/aws/aws-k8s-tester/k8s-tester/splunk`
- Create a file to implement the Tester Interface.   `touch tester.go`
- copy a vend file from another package    `cp ../vend.sh .`
- Write tests
- run  `./vend.sh`
- run  `go mod tidy -v`


# Test/Run singe test stand-alone
```bash
go run cmd/k8s-tester-splunk/main.go apply \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Path/to/kubeconfig" \
    --access-key="1234567890" \
    --splunk-realm="us1" \
    --log-outputs="splunk.log"

## Delete
go run cmd/k8s-tester-splunk/main.go delete \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Path/to/kubeconfig" \
    --access-key="1234567890" \
    --collector-endpoint="us2.app.splunk.com" \
    --log-outputs="splunk.log"
```

# This test is equivilant to running this set of commands

helm repo add splunk-otel-collector-chart https://signalfx.github.io/splunk-otel-collector-chart && helm repo update

---

helm install 
--set provider='aws' 
--set distro='eks' 
--set splunkAccessToken='1234567890' 
--set clusterName='eks-anywhere-cluster' 
--set splunkRealm='us1' 
--set otelCollector.enabled='false'  
--generate-name splunk-otel-collector-chart/splunk-otel-collector

---