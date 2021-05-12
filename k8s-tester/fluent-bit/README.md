# Goals

- Provide the ability to test functionality of different software types.
- Provide impodence on create/delete in case failures occur (Can re-run apply twice without getting multiple resources or errors)


# Requirements

1. Log file
   ```
   touch fluent-bit.log
   ```
2. Kubectl Path
    ```
    which kubectl
    ```
3. Kubeconfig Path
    - Usually this is `~/.kube/config`

# Usage instructions

- namespace: This is the "testing" namespace items will be created in your cluster

### Test

```bash
go run cmd/k8s-tester-fluent-bit/main.go apply \
    --namespace loggingsuite \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/.kube/config" \
    --log-outputs="/Users/jonahjo/go/src/code.amazon.com/aws-k8s-tester/k8s-tester/fluent-bit/log.log"
```

### Delete
```bash
go run cmd/k8s-tester-fluent-bit/main.go delete \
    --namespace loggingsuite \
    --kubectl-path="/usr/local/bin/kubectl" \
    --kubeconfig-path="/Users/jonahjo/.kube/config" \
    --log-outputs="/Users/jonahjo/go/src/code.amazon.com/aws-k8s-tester/k8s-tester/fluent-bit/log.log"
```

